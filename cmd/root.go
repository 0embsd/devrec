package cmd

import (
	"github.com/0embsd/devrec/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Version is set via ldflags at build time.
	Version = "0.1.0"

	cfg     *config.Config
	rootCmd = &cobra.Command{
		Use:   "devrec",
		Short: "Linux test session recorder with structured snapshots",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Load(cmd.Flags())
			if err != nil {
				return err
			}
			cfg = c
			return nil
		},
	}
)

func init() {
	rootCmd.PersistentFlags().String("dir", "", "base directory (default: /opt/devrec)")
	rootCmd.PersistentFlags().String("config", "", "config file path")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// getConfig returns the loaded config. Must be called after PersistentPreRunE.
func getConfig() *config.Config {
	if cfg == nil {
		cfg = config.Defaults()
	}
	return cfg
}
