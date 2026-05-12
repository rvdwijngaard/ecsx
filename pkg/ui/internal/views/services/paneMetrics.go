package serviceselection

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"

	appconfig "github.com/ron/ecsx/pkg"
	cwadapter "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatch"
	cwtypes "github.com/ron/ecsx/pkg/ui/internal/adapters/cloudwatch/types"
	"github.com/ron/ecsx/pkg/ui/internal/messages"
	"github.com/ron/ecsx/pkg/ui/internal/styles"
)

const (
	dsMin = "min"
	dsMax = "max"
	// default dataset is used for avg
)

type metricsPane struct {
	config *appconfig.Config

	// pane's view window
	window struct {
		width  int
		height int
	}

	// separate charts for CPU and Memory
	cpuChart timeserieslinechart.Model
	memChart timeserieslinechart.Model

	// stored data points for display
	cpuSeries cwtypes.MetricSeries
	memSeries cwtypes.MetricSeries

	// current service context
	cluster string
	service string

	// state
	loading bool
	hasData bool
	err     error
}

var (
	// Avg line: bright, prominent
	cpuAvgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))  // blue/purple
	memAvgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")) // pink/magenta

	// Max line: lighter shade
	cpuMaxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("111")) // light blue
	memMaxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("218")) // light pink

	// Min line: dimmer shade
	cpuMinStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("60"))  // dim blue
	memMinStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("132")) // dim pink

	axStyle  = lipgloss.NewStyle().Foreground(styles.SubtleColour)
	lblStyle = lipgloss.NewStyle().Foreground(styles.SubtleColour)
)

func newMetricsPane(config *appconfig.Config) *metricsPane {
	p := &metricsPane{
		config: config,
	}
	p.cpuChart = p.newCPUChart(30, 4)
	p.memChart = p.newMemChart(30, 4)
	return p
}

func (m *metricsPane) newCPUChart(w, h int) timeserieslinechart.Model {
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	chart := timeserieslinechart.New(w, h,
		timeserieslinechart.WithTimeRange(start, now),
		timeserieslinechart.WithAxesStyles(axStyle, lblStyle),
		timeserieslinechart.WithXLabelFormatter(timeserieslinechart.HourTimeLabelFormatter()),
		timeserieslinechart.WithXYSteps(3, 2),
		timeserieslinechart.WithStyle(cpuAvgStyle),                       // default = avg
		timeserieslinechart.WithLineStyle(runes.ArcLineStyle),            // avg: solid
		timeserieslinechart.WithDataSetStyle(dsMax, cpuMaxStyle),          // max: lighter
		timeserieslinechart.WithDataSetLineStyle(dsMax, runes.ArcLineStyle),
		timeserieslinechart.WithDataSetStyle(dsMin, cpuMinStyle),          // min: dimmer
		timeserieslinechart.WithDataSetLineStyle(dsMin, runes.ArcLineStyle),
	)
	return chart
}

func (m *metricsPane) newMemChart(w, h int) timeserieslinechart.Model {
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	chart := timeserieslinechart.New(w, h,
		timeserieslinechart.WithTimeRange(start, now),
		timeserieslinechart.WithAxesStyles(axStyle, lblStyle),
		timeserieslinechart.WithXLabelFormatter(timeserieslinechart.HourTimeLabelFormatter()),
		timeserieslinechart.WithXYSteps(3, 2),
		timeserieslinechart.WithStyle(memAvgStyle),                       // default = avg
		timeserieslinechart.WithLineStyle(runes.ArcLineStyle),            // avg: solid
		timeserieslinechart.WithDataSetStyle(dsMax, memMaxStyle),          // max: lighter
		timeserieslinechart.WithDataSetLineStyle(dsMax, runes.ArcLineStyle),
		timeserieslinechart.WithDataSetStyle(dsMin, memMinStyle),          // min: dimmer
		timeserieslinechart.WithDataSetLineStyle(dsMin, runes.ArcLineStyle),
	)
	return chart
}

func (m *metricsPane) Init() tea.Cmd {
	return nil
}

func (m *metricsPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case messages.ServiceDetails:
		if msg.Details == nil {
			m.hasData = false
			m.loading = false
			m.service = ""
			return nil
		}
		if msg.Details.Name == m.service {
			return nil
		}
		m.service = msg.Details.Name
		return m.loadMetrics()

	case messages.SelectCluster:
		m.cluster = msg.ClusterName
		m.service = ""
		m.hasData = false
		m.loading = false
		return nil

	case messages.ServiceMetricsReady:
		if msg.Service != m.service {
			return nil
		}
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return nil
		}
		m.err = nil
		m.applyMetrics(msg.Metrics)
		return nil
	}
	return nil
}

func (m *metricsPane) loadMetrics() tea.Cmd {
	client := m.config.CloudWatchClient
	if client == nil {
		return nil
	}
	m.loading = true
	m.hasData = false
	cluster := m.cluster
	service := m.service

	return func() tea.Msg {
		ctx := context.Background()
		metrics, err := cwadapter.GetServiceMetrics(client, ctx, cluster, service)
		return messages.ServiceMetricsReady{
			Cluster: cluster,
			Service: service,
			Metrics: metrics,
			Err:     err,
		}
	}
}

func (m *metricsPane) applyMetrics(metrics *cwtypes.ServiceMetrics) {
	if metrics == nil || !metrics.HasData {
		m.hasData = false
		m.cpuSeries = cwtypes.MetricSeries{}
		m.memSeries = cwtypes.MetricSeries{}
		return
	}
	m.hasData = true
	m.cpuSeries = metrics.CPU
	m.memSeries = metrics.Mem

	// Each chart gets half the height
	chartH := max(3, (m.window.height-2)/2)

	// Rebuild charts with current dimensions
	m.cpuChart = m.newCPUChart(m.window.width, chartH)
	m.memChart = m.newMemChart(m.window.width, chartH)

	m.pushSeries(&m.cpuChart, m.cpuSeries)
	m.pushSeries(&m.memChart, m.memSeries)

	m.cpuChart.DrawBrailleAll()
	m.memChart.DrawBrailleAll()
}

func (m *metricsPane) pushSeries(chart *timeserieslinechart.Model, series cwtypes.MetricSeries) {
	for _, pt := range series.Avg {
		chart.Push(timeserieslinechart.TimePoint{Time: pt.Time, Value: pt.Value})
	}
	for _, pt := range series.Max {
		chart.PushDataSet(dsMax, timeserieslinechart.TimePoint{Time: pt.Time, Value: pt.Value})
	}
	for _, pt := range series.Min {
		chart.PushDataSet(dsMin, timeserieslinechart.TimePoint{Time: pt.Time, Value: pt.Value})
	}
}

func (m *metricsPane) applySize(height, width int) {
	m.window.height = height
	m.window.width = width
	if m.hasData {
		chartH := max(3, (height-2)/2)
		m.cpuChart.Resize(width, chartH)
		m.memChart.Resize(width, chartH)
		m.cpuChart.DrawBrailleAll()
		m.memChart.DrawBrailleAll()
	}
}

func (m *metricsPane) View() string {
	if m.service == "" {
		return ""
	}
	if m.loading {
		return lipgloss.NewStyle().
			Foreground(styles.SubtleColour).
			PaddingLeft(1).
			Render("Loading metrics...")
	}
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			PaddingLeft(1).
			Render(fmt.Sprintf("Metrics error: %v", m.err))
	}
	if !m.hasData {
		return lipgloss.NewStyle().
			Foreground(styles.SubtleColour).
			PaddingLeft(1).
			Render("No metrics data available")
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(styles.ViewFocusBorderColour).Render("Metrics (1h)")

	// Show latest avg values alongside labels
	cpuVal := ""
	if pts := m.cpuSeries.Avg; len(pts) > 0 {
		cpuVal = fmt.Sprintf(" %.1f%%", pts[len(pts)-1].Value)
	}
	memVal := ""
	if pts := m.memSeries.Avg; len(pts) > 0 {
		memVal = fmt.Sprintf(" %.1f%%", pts[len(pts)-1].Value)
	}

	cpuLabel := cpuAvgStyle.Render("CPU"+cpuVal) + "  " +
		cpuMaxStyle.Render("max") + " " +
		cpuMinStyle.Render("min")
	memLabel := memAvgStyle.Render("Mem"+memVal) + "  " +
		memMaxStyle.Render("max") + " " +
		memMinStyle.Render("min")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		cpuLabel,
		m.cpuChart.View(),
		memLabel,
		m.memChart.View(),
	)
}
