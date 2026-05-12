package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	appconfig "github.com/ron/ecsx/pkg"
	"github.com/ron/ecsx/pkg/configfile"
	"github.com/ron/ecsx/pkg/ui"
)

var (
	version string
	profile string
	region  string
	cluster string
	verbose bool
	cfgPath string
)

const (
	aws_profile_key = "aws_profile"
	config_key      = "config"
	region_key      = "region"
	cluster_key     = "cluster"

	corrupt_config_dir = "<config_dir_not_found>"
)

var configDir string

func init() {
	var err error
	configDir, err = os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine config directory: %v\n", err)
		configDir = corrupt_config_dir
	}
}

func main() {
	root := &cobra.Command{
		Use:          "ecsx",
		Short:        "ECS terminal UI and log tailer",
		Version:      version,
		SilenceUsage: true,
		RunE:         runApplication,
	}

	root.PersistentFlags().StringVarP(&profile, aws_profile_key, "p", "", "AWS profile")
	root.PersistentFlags().StringVarP(&region, region_key, "r", "", "AWS region")
	root.PersistentFlags().StringVarP(&cfgPath, config_key, "c", filepath.Join(configDir, "ecsx/config.yaml"), "path to config file (relative or absolute); must be yaml")
	root.PersistentFlags().StringVar(&cluster, cluster_key, "", "ECS cluster name")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")

	// root.RegisterFlagCompletionFunc(cluster_key, completeClusters)

	// TODO: re-enable subcommands once migrated to pkg/ connectors
	// root.AddCommand(logsCmd())
	// root.AddCommand(ssmCmd())
	// root.AddCommand(execCmd())
	// root.AddCommand(taskCmd())
	// root.AddCommand(containerEnvCmd())
	// root.AddCommand(completionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runApplication(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	var uiopts []ui.Option

	cfgf, _, err := loadConfig(cfgPath)
	if err != nil {
		uiopts = append(uiopts, ui.WithInitialErrorNotification(err))
	}

	// Set up credentials channel for MFA token provider
	credsC := make(chan appconfig.CredentialsResponse, 1)
	defer close(credsC)

	var p *tea.Program
	mfaCB := func() (string, error) {
		p.Send(appconfig.CredentialsRequest{})
		resp := <-credsC
		return resp.Token, resp.Error
	}

	cfg := appconfig.Config{
		Profile:          resolveProfile(cfgf),
		Region:           resolveRegion(cfgf),
		Cluster:          cluster,
		Verbose:          verbose,
		AvailableRegions: cfgf.AWSRegions,
		StarredRegions:   cfgf.StarredRegions,
		MaxTables:        cfgf.MaxTables,

		MFACredentialCB: mfaCB,
		MFACredentialC:  credsC,
	}

	// AWS config loading and client creation is handled by the TUI's Init()
	// method (see pkg/ui/home.go). This ensures `p` is assigned before MFA
	// callbacks can fire, avoiding a nil-pointer panic.

	p = tea.NewProgram(ui.NewModel(ctx, cfg, uiopts...))
	_, err = p.Run()
	return err
}

func loadConfig(path string) (configfile.ConfigFile, *configfile.ConfigManager, error) {
	full, err1 := filepath.Abs(path)
	if err1 != nil {
		err1 = fmt.Errorf("failed to construct a valid config-path: %w", err1)
	}

	configman := configfile.NewConfigManager(full)
	cfgf, err2 := configman.LoadConfig(true)
	if err1 != nil {
		return cfgf, configman, err1
	}
	if err2 != nil {
		return cfgf, configman, fmt.Errorf("failed to load local config: %w", err2)
	}

	return cfgf, configman, nil
}

func resolveProfile(cfg configfile.ConfigFile) *string {
	if pr := profile; pr != "" {
		return &pr
	}
	if pr := os.Getenv("AWS_PROFILE"); pr != "" {
		return &pr
	}
	if pr := cfg.DefaultProfile; pr != "" {
		return &pr
	}
	return nil
}

func resolveRegion(cfg configfile.ConfigFile) string {
	if r := region; r != "" {
		return r
	}
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	if r := cfg.DefaultRegion; r != "" {
		return r
	}
	return "us-east-1"
}


