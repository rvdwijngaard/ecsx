package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	appconfig "github.com/ron/ecsx/pkg"
	pkgaws "github.com/ron/ecsx/pkg/aws"
	"github.com/ron/ecsx/pkg/aws/ecs"
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

	root.RegisterFlagCompletionFunc(cluster_key, completeClusters)

	// TODO: re-enable subcommands once migrated to pkg/ connectors
	// root.AddCommand(logsCmd())
	// root.AddCommand(ssmCmd())
	// root.AddCommand(execCmd())
	// root.AddCommand(taskCmd())
	// root.AddCommand(containerEnvCmd())
	root.AddCommand(completionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runApplication(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	var uiopts []ui.Option

	// Resolve profile: flag → env → default (nil)
	resolvedProfile := resolveProfile()

	// Resolve region: flag → env → default
	resolvedRegion := resolveRegion()

	// Set up credentials channel for MFA token provider
	credsC := make(chan appconfig.CredentialsResponse, 1)
	var p *tea.Program
	mfaCB := func() (string, error) {
		p.Send(appconfig.CredentialsRequest{})
		resp := <-credsC
		return resp.Token, resp.Error
	}

	cfg := appconfig.Config{
		Profile:         resolvedProfile,
		Region:          resolvedRegion,
		Cluster:         cluster,
		Verbose:         verbose,
		MFACredentialCB: mfaCB,
		MFACredentialC:  credsC,
	}

	// Load AWS config
	awsCfg, err := pkgaws.LoadAWSConfig(ctx, cfg.Region, cfg.Profile, cfg.MFACredentialCB)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ECS client
	profileStr := ""
	if cfg.Profile != nil {
		profileStr = *cfg.Profile
	}
	ecsClient := ecs.NewClient(awsCfg, profileStr)
	cfg.ECSClient = ecsClient.ECS

	p = tea.NewProgram(ui.NewModel(ctx, cfg, uiopts...))
	_, err = p.Run()
	return err
}

func resolveProfile() *string {
	if profile != "" {
		return &profile
	}
	if p := os.Getenv("AWS_PROFILE"); p != "" {
		return &p
	}
	return nil
}

func resolveRegion() string {
	if region != "" {
		return region
	}
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	return "us-east-1"
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Output shell completion script",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}
