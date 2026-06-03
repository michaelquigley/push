package main

import (
	"log/slog"
	"os"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/push/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "push",
	Short: "push - lightweight deployment agent",
	Long:  "Push syncs build artifacts from a shared filesystem depot to local hosts.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			dl.Init(dl.DefaultOptions().SetTrimPrefix("github.com/michaelquigley/").SetLevel(slog.LevelDebug))
		}
		return nil
	},
}

var (
	configPath string
	verbose    bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
}

func main() {
	dl.Init(dl.DefaultOptions().SetTrimPrefix("github.com/michaelquigley/"))
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadConfig loads the push configuration, respecting the --config flag.
func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.LoadFrom(configPath)
	}
	return config.Load()
}
