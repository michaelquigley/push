package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/michaelquigley/df/dd"
)

// Config is the top-level push configuration.
type Config struct {
	Depot       string `dd:",+required"`
	InstallPath string
	Interval    time.Duration
	StateDir    string
	Builds      []BuildConfig `dd:",+required"`
}

// BuildConfig describes a single build to track.
type BuildConfig struct {
	Name        string `dd:",+required"`
	InstallPath string
}

// BuildMeta mirrors build.json in the depot.
type BuildMeta struct {
	SHA     string
	Short   string
	Message string
	Author  string
	Time    string
	Ref     string
	Version string
}

// Load loads config from the default config path.
func Load() (*Config, error) {
	return LoadFrom(DefaultConfigPath())
}

// LoadFrom loads config from the given path.
func LoadFrom(path string) (*Config, error) {
	cfg := &Config{}
	if err := dd.BindYAMLFile(cfg, path); err != nil {
		return nil, fmt.Errorf("loading config from %s: %w", path, err)
	}

	// expand tildes
	var err error
	cfg.StateDir, err = expandTilde(cfg.StateDir)
	if err != nil {
		return nil, fmt.Errorf("expanding state_dir: %w", err)
	}
	cfg.Depot, err = expandTilde(cfg.Depot)
	if err != nil {
		return nil, fmt.Errorf("expanding depot: %w", err)
	}
	cfg.InstallPath, err = expandTilde(cfg.InstallPath)
	if err != nil {
		return nil, fmt.Errorf("expanding install_path: %w", err)
	}
	for i := range cfg.Builds {
		cfg.Builds[i].InstallPath, err = expandTilde(cfg.Builds[i].InstallPath)
		if err != nil {
			return nil, fmt.Errorf("expanding install_path for '%s': %w", cfg.Builds[i].Name, err)
		}
	}

	// inherit global install_path into builds that don't set their own
	for i := range cfg.Builds {
		if cfg.Builds[i].InstallPath == "" {
			cfg.Builds[i].InstallPath = cfg.InstallPath
		}
	}

	// default state dir
	if cfg.StateDir == "" {
		cfg.StateDir = DefaultStateDir()
	}

	// default interval
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}

	// validate
	if len(cfg.Builds) == 0 {
		return nil, fmt.Errorf("at least one build must be configured")
	}
	seen := make(map[string]bool)
	for _, b := range cfg.Builds {
		if seen[b.Name] {
			return nil, fmt.Errorf("duplicate build name: %s", b.Name)
		}
		seen[b.Name] = true
		if b.InstallPath == "" {
			return nil, fmt.Errorf("build '%s' has no install_path and no global install_path is set", b.Name)
		}
	}

	return cfg, nil
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "push", "config.yaml")
}

// DefaultStateDir returns the default state directory path.
func DefaultStateDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "push", "state")
}

// LoadBuildMeta reads a build.json file.
func LoadBuildMeta(path string) (*BuildMeta, error) {
	meta := &BuildMeta{}
	if err := dd.BindJSONFile(meta, path); err != nil {
		return nil, fmt.Errorf("loading build metadata from %s: %w", path, err)
	}
	return meta, nil
}

// expandTilde replaces a leading ~/ with the user's home directory. On Windows, the path is
// returned unchanged because Windows configs use explicit paths.
func expandTilde(path string) (string, error) {
	if path == "" {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		return path, nil
	}
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
