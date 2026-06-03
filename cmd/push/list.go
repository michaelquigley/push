package main

import (
	"fmt"
	"strings"

	"github.com/michaelquigley/push/internal/sync"
	"github.com/spf13/cobra"
)

type listCmd struct {
	cmd *cobra.Command
}

func newListCmd() *listCmd {
	cmd := &cobra.Command{
		Use:   "list [name]",
		Short: "list builds, or versions for a specific build",
		Args:  cobra.MaximumNArgs(1),
	}
	command := &listCmd{cmd: cmd}
	cmd.RunE = command.run
	return command
}

func init() {
	rootCmd.AddCommand(newListCmd().cmd)
}

func (c *listCmd) run(_ *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// no args: print configured build names
	if len(args) == 0 {
		for _, b := range cfg.Builds {
			fmt.Println(b.Name)
		}
		return nil
	}

	name := args[0]
	entries, err := sync.ListBuilds(cfg, name)
	if err != nil {
		return fmt.Errorf("listing builds for %s: %w", name, err)
	}

	if len(entries) == 0 {
		fmt.Printf("no builds found for %s\n", name)
		return nil
	}

	for _, e := range entries {
		// marker: * = latest + active, L = latest, A = active, space = neither
		marker := " "
		if e.IsLatest && e.IsActive {
			marker = "*"
		} else if e.IsLatest {
			marker = "L"
		} else if e.IsActive {
			marker = "A"
		}

		// format time — take first 16 chars of the ISO time for "YYYY-MM-DD HH:MM"
		timeStr := e.Meta.Time
		if len(timeStr) >= 16 {
			timeStr = timeStr[:10] + " " + timeStr[11:16]
		}

		platforms := "[" + strings.Join(e.Platforms, ", ") + "]"

		fmt.Printf("  %s %s  %s  %s  %s\n", marker, e.Meta.Short, timeStr, e.Meta.Message, platforms)
	}

	fmt.Println()
	fmt.Println("  * = latest + active, L = latest in depot, A = active locally")

	return nil
}
