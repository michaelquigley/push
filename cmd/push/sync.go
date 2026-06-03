package main

import (
	"fmt"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/push/internal/sync"
	"github.com/spf13/cobra"
)

type syncCmd struct {
	cmd *cobra.Command
}

func newSyncCmd() *syncCmd {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "sync all builds to latest",
		Args:  cobra.NoArgs,
	}
	command := &syncCmd{cmd: cmd}
	cmd.RunE = command.run
	return command
}

func init() {
	s := newSyncCmd()
	rootCmd.AddCommand(s.cmd)
	// bare `push` = `push sync`
	rootCmd.RunE = s.run
}

func (c *syncCmd) run(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	results := sync.SyncAll(cfg)

	var hasError bool
	for _, r := range results {
		if r.Err != nil {
			dl.Errorf("'%s': %v", r.Name, r.Err)
			hasError = true
		} else if r.Updated {
			dl.Infof("'%s' updated to '%s'", r.Name, r.SHA)
		}
	}

	if hasError {
		return fmt.Errorf("one or more builds failed to sync")
	}
	return nil
}
