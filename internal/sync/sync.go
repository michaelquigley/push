package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/push/internal/config"
	"github.com/michaelquigley/push/internal/platform"
)

// BuildStatus represents the sync state of a build.
type BuildStatus int

const (
	UpToDate BuildStatus = iota
	Outdated
	NotInstalled
	NoPlatform
	Pinned
	Damaged
	Error
)

func (s BuildStatus) String() string {
	switch s {
	case UpToDate:
		return "up to date"
	case Outdated:
		return "outdated"
	case NotInstalled:
		return "not installed"
	case NoPlatform:
		return "no build for " + platform.Detect()
	case Pinned:
		return "pinned"
	case Damaged:
		return "damaged"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// BuildState is the computed state for status display.
type BuildState struct {
	Name      string
	Status    BuildStatus
	LocalSHA  string
	DepotSHA  string
	PinSHA    string
	DepotMeta *config.BuildMeta
	Err       error
}

// SyncResult is the outcome of syncing one build.
type SyncResult struct {
	Name    string
	Updated bool
	SHA     string
	Err     error
}

// ListEntry represents one build version for list display.
type ListEntry struct {
	Meta      *config.BuildMeta
	Platforms []string
	IsLatest  bool
	IsActive  bool
}

// ManifestEntry is a file path with its SHA-256 hash.
type ManifestEntry struct {
	Path string
	Hash string
}

// SyncAll syncs all configured builds. Never short-circuits on error.
func SyncAll(cfg *config.Config) []SyncResult {
	if _, err := os.Stat(cfg.Depot); err != nil {
		dl.Errorf("depot '%s' is not reachable: %v", cfg.Depot, err)
		var results []SyncResult
		for _, build := range cfg.Builds {
			results = append(results, SyncResult{Name: build.Name, Err: fmt.Errorf("depot not reachable: %w", err)})
		}
		return results
	}
	var results []SyncResult
	for _, build := range cfg.Builds {
		results = append(results, SyncOne(cfg, build))
	}
	return results
}

// SyncOne syncs a single build.
func SyncOne(cfg *config.Config, build config.BuildConfig) SyncResult {
	result := SyncResult{Name: build.Name}

	// check pin
	if pinned, pinSHA := isPinned(cfg.StateDir, build.Name); pinned {
		dl.Infof("%s: pinned at %s, skipping", build.Name, pinSHA)
		return result
	}

	// read depot latest
	depotSHA, err := readLatest(cfg.Depot, build.Name)
	if err != nil {
		result.Err = fmt.Errorf("reading latest: %w", err)
		return result
	}

	// read local SHA
	localSHA, err := readLocalSHA(cfg.StateDir, build.Name)
	if err != nil && !os.IsNotExist(err) {
		result.Err = fmt.Errorf("reading local SHA: %w", err)
		return result
	}

	// check if identity matches and installed files are verified
	if localSHA == depotSHA {
		if filesVerified(cfg.StateDir, build.Name, build.InstallPath) {
			dl.Debugf("'%s' up to date at '%s'", build.Name, parseBuildDir(depotSHA))
			return result
		}
		dl.Infof("%s: reinstalling damaged files (%s)", build.Name, parseBuildDir(depotSHA))
	}

	// check platform directory exists
	plat := platform.Detect()
	srcDir := platformDir(cfg.Depot, build.Name, parseBuildDir(depotSHA), plat)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		dl.Infof("%s: no build for %s", build.Name, plat)
		return result
	} else if err != nil {
		result.Err = fmt.Errorf("checking platform dir: %w", err)
		return result
	}

	// read old manifest for cleanup
	oldEntries := readManifest(cfg.StateDir, build.Name)

	// copy tree
	newEntries, err := copyTree(srcDir, build.InstallPath)
	if err != nil {
		result.Err = fmt.Errorf("copying files: %w", err)
		return result
	}

	// clean up old files not in new build
	cleanOldFiles(build.InstallPath, oldEntries, newEntries)

	// write manifest
	if err := writeManifest(cfg.StateDir, build.Name, newEntries); err != nil {
		result.Err = fmt.Errorf("writing manifest: %w", err)
		return result
	}

	// write SHA last — this is the commit point
	if err := writeLocalSHA(cfg.StateDir, build.Name, depotSHA); err != nil {
		result.Err = fmt.Errorf("writing local SHA: %w", err)
		return result
	}

	result.Updated = true
	result.SHA = depotSHA
	return result
}

// StatusAll returns the status of all configured builds.
func StatusAll(cfg *config.Config) []BuildState {
	var states []BuildState
	for _, build := range cfg.Builds {
		states = append(states, StatusOne(cfg, build))
	}
	return states
}

// StatusOne returns the status of a single build.
func StatusOne(cfg *config.Config, build config.BuildConfig) BuildState {
	state := BuildState{Name: build.Name}

	// check pin
	if pinned, pinSHA := isPinned(cfg.StateDir, build.Name); pinned {
		state.Status = Pinned
		state.PinSHA = pinSHA
		state.LocalSHA = pinSHA

		metaPath := buildMetaPath(cfg.Depot, build.Name, pinSHA)
		if meta, err := config.LoadBuildMeta(metaPath); err == nil {
			state.DepotMeta = meta
		}
		return state
	}

	// read depot latest
	depotSHA, err := readLatest(cfg.Depot, build.Name)
	if err != nil {
		state.Status = Error
		state.Err = err
		return state
	}
	state.DepotSHA = parseBuildDir(depotSHA)

	// load depot metadata
	metaPath := buildMetaPath(cfg.Depot, build.Name, parseBuildDir(depotSHA))
	if meta, err := config.LoadBuildMeta(metaPath); err == nil {
		state.DepotMeta = meta
	}

	// check platform
	plat := platform.Detect()
	srcDir := platformDir(cfg.Depot, build.Name, parseBuildDir(depotSHA), plat)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		state.Status = NoPlatform
		return state
	}

	// read local SHA
	localSHA, err := readLocalSHA(cfg.StateDir, build.Name)
	if err != nil {
		if os.IsNotExist(err) {
			state.Status = NotInstalled
			return state
		}
		state.Status = Error
		state.Err = err
		return state
	}
	state.LocalSHA = parseBuildDir(localSHA)

	// compare
	if localSHA == depotSHA {
		if filesVerified(cfg.StateDir, build.Name, build.InstallPath) {
			state.Status = UpToDate
		} else {
			state.Status = Damaged
		}
	} else {
		state.Status = Outdated
	}
	return state
}

// ListBuilds lists available builds in the depot for the given name.
func ListBuilds(cfg *config.Config, name string) ([]ListEntry, error) {
	buildsDir := filepath.Join(cfg.Depot, name, "builds")
	entries, err := os.ReadDir(buildsDir)
	if err != nil {
		return nil, fmt.Errorf("reading builds directory: %w", err)
	}

	// read latest SHA and local SHA for markers
	latestSHA, _ := readLatest(cfg.Depot, name)
	localSHA, _ := readLocalSHA(cfg.StateDir, name)

	var list []ListEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sha := entry.Name()

		metaPath := buildMetaPath(cfg.Depot, name, sha)
		meta, err := config.LoadBuildMeta(metaPath)
		if err != nil {
			dl.Warnf("skipping build %s: %v", sha, err)
			continue
		}

		// discover platforms
		platforms := discoverPlatforms(filepath.Join(buildsDir, sha))

		list = append(list, ListEntry{
			Meta:      meta,
			Platforms: platforms,
			IsLatest:  sha == parseBuildDir(latestSHA),
			IsActive:  sha == parseBuildDir(localSHA),
		})
	}

	// sort by time descending
	sort.Slice(list, func(i, j int) bool {
		return list[i].Meta.Time > list[j].Meta.Time
	})

	return list, nil
}

// discoverPlatforms returns the list of platform subdirectories for a build.
func discoverPlatforms(buildDir string) []string {
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return nil
	}
	var platforms []string
	for _, entry := range entries {
		if entry.IsDir() {
			platforms = append(platforms, entry.Name())
		}
	}
	sort.Strings(platforms)
	return platforms
}

// --- internal helpers ---

func readLatest(depot, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(depot, name, "latest"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readLocalSHA(stateDir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, name+".sha"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeLocalSHA(stateDir, name, sha string) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, name+".sha"), []byte(sha+"\n"), 0644)
}

func isPinned(stateDir, name string) (bool, string) {
	data, err := os.ReadFile(filepath.Join(stateDir, name+".pin"))
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(string(data))
}

func readPinSHA(stateDir, name string) string {
	_, sha := isPinned(stateDir, name)
	return sha
}

func readManifest(stateDir, name string) []ManifestEntry {
	data, err := os.ReadFile(filepath.Join(stateDir, name+".files"))
	if err != nil {
		return nil
	}
	var entries []ManifestEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, parseManifestLine(line))
	}
	return entries
}

// parseManifestLine parses a single manifest line. New format is "<64-hex>  <path>"; legacy
// format is just a path. Legacy entries get an empty hash, which forces reinstall.
func parseManifestLine(line string) ManifestEntry {
	if len(line) > 66 && line[64:66] == "  " {
		prefix := line[:64]
		if _, err := hex.DecodeString(prefix); err == nil {
			return ManifestEntry{
				Path: filepath.FromSlash(line[66:]),
				Hash: prefix,
			}
		}
	}
	return ManifestEntry{Path: filepath.FromSlash(line)}
}

func writeManifest(stateDir, name string, entries []ManifestEntry) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	var buf strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&buf, "%s  %s\n", e.Hash, e.Path)
	}
	return os.WriteFile(filepath.Join(stateDir, name+".files"), []byte(buf.String()), 0644)
}

// copyTree walks srcDir and copies all files to dstDir, returning manifest entries.
func copyTree(srcDir, dstDir string) ([]ManifestEntry, error) {
	var entries []ManifestEntry
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dst := filepath.Join(dstDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}

		hash, err := copyFile(path, dst)
		if err != nil {
			return fmt.Errorf("copying %s: %w", rel, err)
		}

		entries = append(entries, ManifestEntry{Path: filepath.ToSlash(rel), Hash: hash})
		return nil
	})
	return entries, err
}

// copyFile copies src to dst atomically: temp file + chmod + rename. Returns the SHA-256 hash
// of the copied content.
func copyFile(src, dst string) (string, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return "", err
	}

	// ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}

	// write to temp file in target directory
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".push-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer func() {
		// clean up temp file on error
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	// hash while copying — zero extra I/O
	h := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(srcFile, h)); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	// set permissions: 0755 for executables, 0644 for everything else. On Windows, skip chmod
	// entirely since file extension determines executability.
	if runtime.GOOS != "windows" {
		perm := os.FileMode(0644)
		if srcInfo.Mode()&0111 != 0 {
			perm = 0755
		}
		if err := os.Chmod(tmpPath, perm); err != nil {
			return "", err
		}
	}

	// atomic rename
	if err := os.Rename(tmpPath, dst); err != nil {
		return "", err
	}
	tmpPath = "" // prevent deferred cleanup
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cleanOldFiles removes files from oldEntries that are not in newEntries.
func cleanOldFiles(installPath string, oldEntries, newEntries []ManifestEntry) {
	newSet := make(map[string]bool, len(newEntries))
	for _, e := range newEntries {
		newSet[e.Path] = true
	}
	for _, e := range oldEntries {
		if !newSet[e.Path] {
			path := filepath.Join(installPath, e.Path)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				dl.Warnf("cleaning up %s: %v", e.Path, err)
			} else if err == nil {
				dl.Infof("removed old file: %s", e.Path)
			}
		}
	}
}

// hashFile returns the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
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

// filesVerified checks whether all files from the manifest exist and match their stored hashes.
func filesVerified(stateDir, name, installPath string) bool {
	entries := readManifest(stateDir, name)
	if len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e.Hash == "" {
			dl.Debugf("%s: legacy manifest entry, reinstall needed", e.Path)
			return false
		}
		path := filepath.Join(installPath, e.Path)
		got, err := hashFile(path)
		if err != nil {
			dl.Debugf("%s: cannot read file: %v", e.Path, err)
			return false
		}
		if got != e.Hash {
			dl.Debugf("%s: hash mismatch (expected %s, got %s)", e.Path, e.Hash, got)
			return false
		}
	}
	return true
}

// parseBuildDir extracts the build directory name (short SHA) from an identity string.
// the identity format is "{SHORT}.{UNIX_EPOCH}"; a bare SHA (no dot) is returned as-is
// for backward compatibility.
func parseBuildDir(identity string) string {
	if i := strings.IndexByte(identity, '.'); i > 0 {
		return identity[:i]
	}
	return identity
}

func platformDir(depot, name, sha, plat string) string {
	return filepath.Join(depot, name, "builds", sha, plat)
}

func buildMetaPath(depot, name, sha string) string {
	return filepath.Join(depot, name, "builds", sha, "build.json")
}

// suppress unused warning — readPinSHA is available for future use
var _ = readPinSHA
