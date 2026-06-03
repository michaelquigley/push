package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/push/internal/platform"
	"github.com/spf13/cobra"
)

type vendorCmd struct {
	cmd     *cobra.Command
	version string
	message string
}

func newVendorCmd() *vendorCmd {
	cmd := &cobra.Command{
		Use:   "vendor <binary> <name>",
		Short: "stage a third-party binary into the depot",
		Args:  cobra.ExactArgs(2),
	}
	command := &vendorCmd{cmd: cmd}
	cmd.RunE = command.run
	cmd.Flags().StringVar(&command.version, "version", "", "version label (default: short hash)")
	cmd.Flags().StringVar(&command.message, "message", "", "build message (default: 'vendored')")
	return command
}

func init() {
	rootCmd.AddCommand(newVendorCmd().cmd)
}

func (c *vendorCmd) run(_ *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	binaryPath := args[0]
	depotName := args[1]

	// expand leading ~/
	if strings.HasPrefix(binaryPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		binaryPath = filepath.Join(home, binaryPath[2:])
	}

	// validate source
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("stat '%s': %w", binaryPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("'%s' is not a regular file", binaryPath)
	}

	// hash the binary
	fullHash, err := hashBinary(binaryPath)
	if err != nil {
		return fmt.Errorf("hashing '%s': %w", binaryPath, err)
	}
	short := fullHash[:8]
	dl.Infof("sha-256 of '%s': %s (short '%s')", binaryPath, fullHash, short)

	// check if depot already has this version as latest
	latestPath := filepath.Join(cfg.Depot, depotName, "latest")
	if data, err := os.ReadFile(latestPath); err == nil {
		current := strings.TrimSpace(string(data))
		if strings.HasPrefix(current, short+".") {
			dl.Infof("'%s' already at current version '%s'", depotName, short)
			return nil
		}
	}

	// create platform directory
	plat := platform.Detect()
	platDir := filepath.Join(cfg.Depot, depotName, "builds", short, plat)
	if err := os.MkdirAll(platDir, 0755); err != nil {
		return fmt.Errorf("creating platform directory: %w", err)
	}

	// atomic copy into platform directory
	dstPath := filepath.Join(platDir, depotName)
	if err := atomicCopy(binaryPath, dstPath); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}
	dl.Infof("installed '%s' to '%s'", binaryPath, dstPath)

	// write build.json
	author := os.Getenv("USER")
	if author == "" {
		author = "unknown"
	}

	msg := c.message
	if msg == "" {
		msg = "vendored"
	}
	ver := c.version
	if ver == "" {
		ver = short
	}

	meta := struct {
		SHA     string `json:"SHA"`
		Short   string `json:"Short"`
		Message string `json:"Message"`
		Author  string `json:"Author"`
		Time    string `json:"Time"`
		Ref     string `json:"Ref"`
		Version string `json:"Version"`
	}{
		SHA:     fullHash,
		Short:   short,
		Message: msg,
		Author:  author,
		Time:    time.Now().Format(time.RFC3339),
		Ref:     "vendor",
		Version: ver,
	}

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling build metadata: %w", err)
	}
	metaPath := filepath.Join(cfg.Depot, depotName, "builds", short, "build.json")
	if err := os.WriteFile(metaPath, append(metaBytes, '\n'), 0644); err != nil {
		return fmt.Errorf("writing build.json: %w", err)
	}
	dl.Infof("wrote '%s'", metaPath)

	// write latest
	identity := fmt.Sprintf("%s.%d", short, time.Now().Unix())
	if err := os.WriteFile(latestPath, []byte(identity+"\n"), 0644); err != nil {
		return fmt.Errorf("writing latest: %w", err)
	}
	dl.Infof("updated '%s' to '%s'", latestPath, identity)

	return nil
}

// hashBinary returns the hex-encoded SHA-256 hash of the file at path.
func hashBinary(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// atomicCopy copies src to dst using a temp file + rename pattern, setting mode 0755.
func atomicCopy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".push-vendor-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, srcFile); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}
	tmpPath = "" // prevent deferred cleanup
	return nil
}
