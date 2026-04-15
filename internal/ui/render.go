package ui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	ecsaws "github.com/ron/ecsx/internal/aws"
	"github.com/ron/ecsx/internal/logs"
)

func (m *Model) breadcrumb() string {
	parts := []string{"clusters"}
	if m.clusterName != "" {
		parts = append(parts, m.clusterName)
	}
	if m.serviceName != "" {
		parts = append(parts, m.serviceName)
	}
	if m.showEnvVars {
		parts = append(parts, "env")
	}
	if m.level == viewLogs {
		parts = append(parts, "logs")
	}
	if m.level == viewEC2Instances {
		parts = append(parts, "ec2")
	}
	return breadcrumbStyle.Render(strings.Join(parts, " › "))
}

func (m *Model) updateDetail() {
	if m.confirming {
		switch m.confirmAction {
		case "deploy":
			var b strings.Builder
			fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("Force New Deployment"))
			fmt.Fprintf(&b, "  Service: %s\n\n", m.scaleSvc)
			fmt.Fprintf(&b, "  %s\n\n", warnStyle.Render("⚠ This will trigger a rolling restart of all tasks."))
			fmt.Fprintf(&b, "  %s\n", helpStyle.Render("enter confirm • esc cancel"))
			m.detail.SetContent(b.String())
			return
		case "stop":
			sel := m.list.SelectedItem()
			if sel == nil {
				return
			}
			t := sel.(taskItem).task
			var b strings.Builder
			fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("Stop Task"))
			fmt.Fprintf(&b, "  Task: %s\n\n", t.ID)
			fmt.Fprintf(&b, "  %s\n\n", warnStyle.Render("⚠ This will stop the task immediately."))
			fmt.Fprintf(&b, "  %s\n", helpStyle.Render("enter confirm • esc cancel"))
			m.detail.SetContent(b.String())
			return
		default:
			sel := m.list.SelectedItem()
			if sel == nil {
				return
			}
			svc := sel.(serviceItem).service
			if svc == nil {
				return
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("Confirm Scale "+svc.Name))
			fmt.Fprintf(&b, "  %d → %d tasks\n\n", svc.DesiredCount, m.scaleCount)
			if m.scaleCount == 0 {
				fmt.Fprintf(&b, "  %s\n\n", warnStyle.Render("⚠ This will stop all running tasks!"))
			}
			fmt.Fprintf(&b, "  %s\n", helpStyle.Render("enter confirm • esc cancel"))
			m.detail.SetContent(b.String())
			return
		}
	}
	if m.scaling {
		sel := m.list.SelectedItem()
		if sel == nil {
			return
		}
		svc := sel.(serviceItem).service
		if svc == nil {
			return
		}
		m.detail.SetContent(fmt.Sprintf(
			"%s\n\n  Current desired count: %d\n\n  New desired count: %s\n\n  %s",
			titleStyle.Render("Scale "+svc.Name),
			svc.DesiredCount,
			m.scaleInput.View(),
			helpStyle.Render("enter confirm • esc cancel"),
		))
		return
	}
	if m.showEnvVars && m.envVars != nil {
		m.renderEnvVars()
		return
	}
	sel := m.list.SelectedItem()
	if sel == nil {
		if m.level == viewLogs {
			m.renderLogs()
			return
		}
		m.detail.SetContent("  No items")
		return
	}
	if m.level == viewLogDetail {
		m.renderLogSnapshot()
		return
	}
	switch m.level {
	case viewClusters:
		m.renderClusterDetail(sel.(clusterItem).cluster)
	case viewServices:
		si := sel.(serviceItem)
		if si.service == nil {
			m.detail.SetContent(fmt.Sprintf("%s\n\n  %s", titleStyle.Render(si.name), helpStyle.Render("Loading details...")))
		} else {
			m.renderServiceDetail(*si.service)
		}
	case viewTasks:
		m.renderTaskDetail(sel.(taskItem).task)
	case viewEC2Instances:
		m.renderEC2Detail(sel.(ec2Item).instance)
	case viewContainerSelect:
		g := sel.(containerLogItem).group
		m.detail.SetContent(fmt.Sprintf("%s\n\n  %s\n  %s",
			titleStyle.Render("Select container"),
			g.Container,
			helpStyle.Render(g.LogGroup)))
	case viewLogs:
		m.renderLogs()
	}
}

func (m *Model) renderClusterDetail(c ecsaws.Cluster) {
	region := m.client.Region()
	link := fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/home?region=%s#/clusters/%s", region, region, c.Name)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(c.Name))
	kv(&b, "Status", c.Status)
	kv(&b, "Container Instances", fmt.Sprintf("%d", c.ContainerInstances))
	kv(&b, "Active Services", fmt.Sprintf("%d", c.ActiveServices))
	kv(&b, "Running Tasks", fmt.Sprintf("%d", c.RunningTasks))
	kv(&b, "Pending Tasks", fmt.Sprintf("%d", c.PendingTasks))
	m.writeLink(&b, "Console:", link)
	m.detail.SetContent(b.String())
}

func (m *Model) renderServiceDetail(s ecsaws.Service) {
	region := m.client.Region()
	link := fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/home?region=%s#/clusters/%s/services/%s/tasks",
		region, region, m.clusterName, s.Name)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(s.Name))
	kv(&b, "Status", s.Status)
	kv(&b, "Launch Type", s.LaunchType)
	kv(&b, "Task Definition", s.TaskDefinition)
	kv(&b, "Desired Count", fmt.Sprintf("%d", s.DesiredCount))
	kv(&b, "Running Count", fmt.Sprintf("%d", s.RunningCount))
	kv(&b, "Pending Count", fmt.Sprintf("%d", s.PendingCount))

	if s.CreatedAt != nil {
		kv(&b, "Created At", s.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	}

	if metrics, ok := m.metricsMap[s.Name]; ok && metrics.HasData {
		fmt.Fprintf(&b, "\n%s\n", titleStyle.Render("Metrics (1h)"))
		if len(metrics.CPU) > 0 {
			last := metrics.CPU[len(metrics.CPU)-1]
			fmt.Fprintf(&b, "  CPU  %s %5.1f%%\n", sparkline(metrics.CPU), last)
		}
		if len(metrics.Mem) > 0 {
			last := metrics.Mem[len(metrics.Mem)-1]
			fmt.Fprintf(&b, "  Mem  %s %5.1f%%\n", sparkline(metrics.Mem), last)
		}
	} else if _, ok := m.metricsMap[s.Name]; !ok {
		fmt.Fprintf(&b, "\n  %s\n", helpStyle.Render("Loading metrics..."))
	}

	m.writeLink(&b, "Console:", link)
	m.detail.SetContent(b.String())
}

func (m *Model) renderTaskDetail(t ecsaws.Task) {
	region := m.client.Region()
	taskLink := fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/home?region=%s#/clusters/%s/tasks/%s",
		region, region, m.clusterName, t.ID)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("Task "+t.ID[:min(12, len(t.ID))]))
	kv(&b, "Task ID", t.ID)
	kv(&b, "Status", t.Status)
	kv(&b, "Desired Status", t.DesiredStatus)
	kv(&b, "Health Status", t.HealthStatus)
	kv(&b, "Launch Type", t.LaunchType)
	kv(&b, "Task Definition", t.TaskDefinition)
	if t.Group != "" {
		kv(&b, "Group", t.Group)
	}
	if t.CPU != "" {
		kv(&b, "CPU / Memory", t.CPU+" / "+t.Memory)
	}
	if t.StartedAt != nil {
		kv(&b, "Started At", t.StartedAt.Local().Format("2006-01-02 15:04:05"))
		kv(&b, "Uptime", formatDuration(time.Since(*t.StartedAt)))
	}
	if t.CreatedAt != nil {
		kv(&b, "Created At", t.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if t.StoppedAt != nil {
		kv(&b, "Stopped At", t.StoppedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if t.StoppedReason != "" {
		kv(&b, "Stopped Reason", t.StoppedReason)
	}

	if t.LaunchType == "EC2" {
		if t.EC2InstanceID != "" {
			kv(&b, "EC2 Instance", t.EC2InstanceID)
			ec2Link := fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
				region, region, t.EC2InstanceID)
			m.writeLink(&b, "EC2 Console:", ec2Link)
		}
	} else {
		if t.PrivateIP != "" {
			kv(&b, "Private IP", t.PrivateIP)
		}
		if t.PublicIP != "" {
			kv(&b, "Public IP", t.PublicIP)
		}
	}

	m.writeLink(&b, "Task Console:", taskLink)

	if len(t.Containers) > 0 {
		fmt.Fprintf(&b, "\n%s\n", titleStyle.Render("Containers"))
		for _, c := range t.Containers {
			fmt.Fprintf(&b, "\n  %s\n", lipgloss.NewStyle().Bold(true).Render(c.Name))
			fmt.Fprintf(&b, "    Status: %s\n", c.Status)
			if c.HealthStatus != "" && c.HealthStatus != "UNKNOWN" {
				fmt.Fprintf(&b, "    Health: %s\n", c.HealthStatus)
			}
			if c.Image != "" {
				fmt.Fprintf(&b, "    Image: %s\n", c.Image)
			}
			if c.ExitCode != nil {
				fmt.Fprintf(&b, "    Exit Code: %d\n", *c.ExitCode)
			}
			if c.Reason != "" {
				fmt.Fprintf(&b, "    Reason: %s\n", c.Reason)
			}
			for _, nb := range c.NetworkBindings {
				fmt.Fprintf(&b, "    Port: %d→%d (%s)\n", nb.ContainerPort, nb.HostPort, nb.Protocol)
			}
		}
	}
	m.detail.SetContent(b.String())
}

func (m *Model) renderEC2Detail(ci ecsaws.ContainerInstance) {
	region := m.client.Region()
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(ci.EC2InstanceID))
	kv(&b, "Status", ci.Status)
	kv(&b, "Running Tasks", fmt.Sprintf("%d", ci.RunningTasks))
	kv(&b, "Pending Tasks", fmt.Sprintf("%d", ci.PendingTasks))
	ec2Link := fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		region, region, ci.EC2InstanceID)
	m.writeLink(&b, "EC2 Console:", ec2Link)
	fmt.Fprintf(&b, "\n  %s\n", helpStyle.Render("Press enter to start SSM session"))
	m.detail.SetContent(b.String())
}

func (m *Model) renderEnvVars() {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("Environment Variables"))
	for _, cd := range m.envVars {
		fmt.Fprintf(&b, "\n%s\n", lipgloss.NewStyle().Bold(true).PaddingLeft(1).Render(cd.Name))
		if len(cd.EnvVars) == 0 {
			fmt.Fprintf(&b, "  (no environment variables)\n")
			continue
		}
		maxLen := 0
		for _, ev := range cd.EnvVars {
			if len(ev.Name) > maxLen {
				maxLen = len(ev.Name)
			}
		}
		for _, ev := range cd.EnvVars {
			fmt.Fprintf(&b, "  %-*s  %s\n", maxLen, ev.Name, ev.Value)
		}
	}
	m.detail.SetContent(b.String())
}

var logStreamColors = []color.Color{
	lipgloss.Color("69"), lipgloss.Color("212"), lipgloss.Color("114"), lipgloss.Color("208"),
	lipgloss.Color("39"), lipgloss.Color("168"), lipgloss.Color("156"), lipgloss.Color("203"),
}

func (m *Model) renderLogs() {
	var b strings.Builder
	levelTag := ""
	if m.logLevel != logs.LevelAll {
		levelTag = " [" + m.logLevel.String() + "]"
	}
	if m.logGrepping {
		fmt.Fprintf(&b, "%s  %s\n\n", titleStyle.Render("Grep:"), m.logGrepInput.View())
	} else if m.logFiltering {
		fmt.Fprintf(&b, "%s  %s\n\n", titleStyle.Render("Filter:"), m.logFilterInput.View())
	} else {
		tailStatus := "tailing"
		if m.logPinned {
			tailStatus = "paused"
		}
		tags := "Logs (" + tailStatus + ")" + levelTag
		var hints []string
		if m.logFilter != "" {
			hints = append(hints, "filter: "+m.logFilter)
		}
		if m.logGrep != "" {
			hints = append(hints, "grep: /"+m.logGrep+"/")
		}
		if len(hints) > 0 {
			fmt.Fprintf(&b, "%s %s\n\n", titleStyle.Render(tags), helpStyle.Render(strings.Join(hints, " • ")))
		} else {
			fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(tags))
		}
	}
	if len(m.logLines) == 0 {
		fmt.Fprintf(&b, "  %s\n", helpStyle.Render("Waiting for log events..."))
	} else {
		streamColorMap := make(map[string]color.Color)
		colorIdx := 0
		for _, ev := range m.logLines {
			if !m.logLevel.Matches(ev.Message) {
				continue
			}
			if m.logGrepRe != nil && !m.logGrepRe.MatchString(ev.Message) {
				continue
			}
			if _, ok := streamColorMap[ev.Stream]; !ok {
				streamColorMap[ev.Stream] = logStreamColors[colorIdx%len(logStreamColors)]
				colorIdx++
			}
			sc := streamColorMap[ev.Stream]
			ts := lipgloss.NewStyle().Foreground(sc).Render(ev.Timestamp.Local().Format("15:04:05"))
			fmt.Fprintf(&b, "  %s %s\n", ts, ev.Message)
		}
	}
	m.detail.SetContent(b.String())
}

func (m *Model) renderLogDetail() {
	if m.logCursor < 0 || m.logCursor >= len(m.logSnapshot) {
		m.logDetailView.SetContent("")
		return
	}
	ev := m.logSnapshot[m.logCursor]
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("Log Detail"))
	kv(&b, "Timestamp", ev.Timestamp.Local().Format("2006-01-02 15:04:05.000"))
	kv(&b, "Stream", ev.Stream)
	fmt.Fprintf(&b, "\n")
	msg := strings.TrimSpace(ev.Message)
	var raw json.RawMessage
	if json.Unmarshal([]byte(msg), &raw) == nil {
		if pretty, err := json.MarshalIndent(raw, "  ", "  "); err == nil {
			fmt.Fprintf(&b, "  %s\n", string(pretty))
			m.logDetailView.SetContent(b.String())
			return
		}
	}
	fmt.Fprintf(&b, "  %s\n", msg)
	m.logDetailView.SetContent(b.String())
}

func (m *Model) renderLogSnapshot() {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(fmt.Sprintf("Log Browser (%d/%d)", m.logCursor+1, len(m.logSnapshot))))
	streamColorMap := make(map[string]color.Color)
	colorIdx := 0
	for i, ev := range m.logSnapshot {
		if _, ok := streamColorMap[ev.Stream]; !ok {
			streamColorMap[ev.Stream] = logStreamColors[colorIdx%len(logStreamColors)]
			colorIdx++
		}
		sc := streamColorMap[ev.Stream]
		ts := lipgloss.NewStyle().Foreground(sc).Render(ev.Timestamp.Local().Format("15:04:05"))
		if i == m.logCursor {
			line := fmt.Sprintf("▸ %s %s", ev.Timestamp.Local().Format("15:04:05"), ev.Message)
			fmt.Fprintf(&b, "%s\n", lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("255")).Render(line))
		} else {
			fmt.Fprintf(&b, "  %s %s\n", ts, ev.Message)
		}
	}
	m.detail.SetContent(b.String())
}

func (m *Model) renderHelp() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("Keybindings"))
	fmt.Fprintf(&b, "  %-16s %s\n", "↑/k", "Move up")
	fmt.Fprintf(&b, "  %-16s %s\n", "↓/j", "Move down")
	fmt.Fprintf(&b, "  %-16s %s\n", "enter", "Select / drill down")
	fmt.Fprintf(&b, "  %-16s %s\n", "esc", "Go back")
	fmt.Fprintf(&b, "  %-16s %s\n", "/", "Filter list")
	fmt.Fprintf(&b, "  %-16s %s\n", "e", "Toggle environment variables")
	fmt.Fprintf(&b, "  %-16s %s\n", "l", "Tail CloudWatch logs")
	fmt.Fprintf(&b, "  %-16s %s\n", "/ (in logs)", "Filter log lines")
	fmt.Fprintf(&b, "  %-16s %s\n", "e (in logs)", "Open logs in $EDITOR")
	fmt.Fprintf(&b, "  %-16s %s\n", "f (in logs)", "Cycle log level filter (ALL/INF/WARN/ERR)")
	fmt.Fprintf(&b, "  %-16s %s\n", "g (in logs)", "Regex grep filter")
	fmt.Fprintf(&b, "  %-16s %s\n", "s", "Scale service (set desired count)")
	fmt.Fprintf(&b, "  %-16s %s\n", "x", "Action: deploy (services) / stop (tasks) / SSM (clusters)")
	fmt.Fprintf(&b, "  %-16s %s\n", "r", "Refresh (purge cache)")
	fmt.Fprintf(&b, "  %-16s %s\n", "a", "Toggle auto-refresh (30s)")
	fmt.Fprintf(&b, "  %-16s %s\n", "y", "Yank env vars to clipboard")
	fmt.Fprintf(&b, "  %-16s %s\n", "+/-", "Toggle zoom (fullscreen detail)")
	fmt.Fprintf(&b, "  %-16s %s\n", "?", "Toggle this help")
	fmt.Fprintf(&b, "  %-16s %s\n", "q", "Quit")
	fmt.Fprintf(&b, "\n%s\n\n", titleStyle.Render("Navigation"))
	fmt.Fprintf(&b, "  Clusters → Services → Tasks\n")
	fmt.Fprintf(&b, "  Press enter to drill down, esc to go back\n")
	fmt.Fprintf(&b, "\n%s\n\n", titleStyle.Render("Views"))
	fmt.Fprintf(&b, "  The right panel shows details for the selected item.\n")
	fmt.Fprintf(&b, "  Press 'e' on a service or task to view env vars.\n")
	fmt.Fprintf(&b, "  Console links are shown in the detail panel.\n")
	return b.String()
}

func (m Model) helpText() string {
	parts := []string{"↑↓ navigate", "/ filter"}
	if m.level == viewClusters {
		parts = append(parts, "enter select")
	} else if m.level == viewLogs {
		parts = append(parts, "esc back", "↑↓ scroll", "e editor", "f level:"+m.logLevel.String(), "g grep", "/ filter")
		if m.logPinned {
			parts = append(parts, "end resume")
		}
	} else {
		parts = append(parts, "enter select", "esc back", "e env vars", "l logs")
	}
	if m.level == viewServices {
		parts = append(parts, "s scale", "x deploy")
	}
	if m.level == viewTasks {
		parts = append(parts, "x stop")
	}
	if m.showEnvVars {
		parts = append(parts, "y yank")
	}
	if m.level == viewClusters {
		parts = append(parts, "x ssm")
	}
	parts = append(parts, "r refresh")
	if m.autoRefresh {
		parts = append(parts, "a auto:on")
	} else {
		parts = append(parts, "a auto:off")
	}
	parts = append(parts, "+/- zoom", "? help", "q quit")
	return "  " + strings.Join(parts, " • ")
}

func kv(b *strings.Builder, key, val string) {
	label := labelStyle.Render(fmt.Sprintf("  %-22s", key))
	fmt.Fprintf(b, "%s %s\n", label, val)
}

func (m *Model) writeLink(b *strings.Builder, label, url string) {
	maxW := m.detailWidth() - 8
	if maxW > 0 && len(url) > maxW {
		url = url[:maxW] + "…"
	}
	fmt.Fprintf(b, "\n  %s\n  %s\n", label, url)
}
