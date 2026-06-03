package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/push/build"
	"github.com/michaelquigley/push/internal/config"
	"github.com/michaelquigley/push/internal/sync"
	"github.com/spf13/cobra"
)

type daemonCmd struct {
	cmd *cobra.Command
}

func newDaemonCmd() *daemonCmd {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "run continuously on configured interval",
		Args:  cobra.NoArgs,
	}
	command := &daemonCmd{cmd: cmd}
	cmd.RunE = command.run
	return command
}

func init() {
	rootCmd.AddCommand(newDaemonCmd().cmd)
}

// selfBuild returns the build name whose installed binary path matches exe, or "" if none match.
func selfBuild(cfg *config.Config, exe string) string {
	for _, b := range cfg.Builds {
		candidate := filepath.Join(b.InstallPath, b.Name)
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil && exe == resolved {
			return b.Name
		}
	}
	return ""
}

func (c *daemonCmd) run(_ *cobra.Command, _ []string) error {
	dl.Infof("push %s", build.String())

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// resolve our own executable path for self-update detection
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving executable symlinks: %w", err)
	}

	selfName := selfBuild(cfg, exe)
	if selfName != "" {
		dl.Infof("self-update tracking build %q", selfName)
	} else {
		dl.Debugf("no build matches executable '%s', self-update detection disabled", exe)
	}

	// capture initial mtime to detect external updates (e.g. manual 'push sync')
	exeInfo, err := os.Stat(exe)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}
	exeMtime := exeInfo.ModTime()

	dl.Infof("starting daemon (interval: %s)", cfg.Interval)

	// initial sync
	if selfUpdated(sync.SyncAll(cfg), selfName) {
		dl.Infof("push binary updated, exiting for restart")
		return nil
	}
	if info, err := os.Stat(exe); err == nil && info.ModTime() != exeMtime {
		dl.Infof("push binary modified externally, exiting for restart")
		return nil
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			// reload config on each tick
			if newCfg, err := loadConfig(); err != nil {
				dl.Errorf("reloading config: %v (using previous config)", err)
			} else {
				if newCfg.Interval != cfg.Interval {
					dl.Infof("interval changed: %s -> %s", cfg.Interval, newCfg.Interval)
					ticker.Reset(newCfg.Interval)
				}
				cfg = newCfg
				selfName = selfBuild(cfg, exe)
			}

			if selfUpdated(sync.SyncAll(cfg), selfName) {
				dl.Infof("push binary updated, exiting for restart")
				return nil
			}
			if info, err := os.Stat(exe); err == nil && info.ModTime() != exeMtime {
				dl.Infof("push binary modified externally, exiting for restart")
				return nil
			}

		case s := <-sig:
			dl.Infof("received %s, shutting down", s)
			return nil
		}
	}
}

// selfUpdated logs sync results and reports whether the named build was updated.
func selfUpdated(results []sync.SyncResult, selfName string) bool {
	updated := false
	for _, r := range results {
		if r.Err != nil {
			dl.Errorf("'%s': %v", r.Name, r.Err)
		} else if r.Updated {
			dl.Infof("'%s' updated to '%s'", r.Name, r.SHA)
			if r.Name == selfName {
				updated = true
			}
		}
	}
	return updated
}
