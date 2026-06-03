package main

import (
	"fmt"

	"github.com/michaelquigley/push/internal/sync"
	"github.com/spf13/cobra"
)

type statusCmd struct {
	cmd *cobra.Command
}

func newStatusCmd() *statusCmd {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "show sync status for all builds",
		Args:  cobra.NoArgs,
	}
	command := &statusCmd{cmd: cmd}
	cmd.RunE = command.run
	return command
}

func init() {
	rootCmd.AddCommand(newStatusCmd().cmd)
}

func (c *statusCmd) run(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	states := sync.StatusAll(cfg)

	for _, s := range states {
		switch s.Status {
		case sync.UpToDate:
			msg := s.LocalSHA
			if s.DepotMeta != nil {
				msg += " (" + s.DepotMeta.Message + ")"
			}
			fmt.Printf("%-14s up to date %s\n", s.Name, msg)

		case sync.Outdated:
			msg := s.LocalSHA + " -> " + s.DepotSHA
			if s.DepotMeta != nil {
				msg += " (" + s.DepotMeta.Message + ")"
			}
			fmt.Printf("%-14s outdated %s\n", s.Name, msg)

		case sync.Pinned:
			msg := "at " + s.PinSHA
			if s.DepotMeta != nil {
				msg += " (" + s.DepotMeta.Message + ")"
			}
			fmt.Printf("%-14s pinned %s\n", s.Name, msg)

		case sync.Damaged:
			msg := s.LocalSHA
			if s.DepotMeta != nil {
				msg += " (" + s.DepotMeta.Message + ")"
			}
			fmt.Printf("%-14s damaged %s\n", s.Name, msg)

		case sync.NotInstalled:
			fmt.Printf("%-14s not installed\n", s.Name)

		case sync.NoPlatform:
			fmt.Printf("%-14s %s\n", s.Name, s.Status)

		case sync.Error:
			fmt.Printf("%-14s error: %v\n", s.Name, s.Err)
		}
	}

	return nil
}
