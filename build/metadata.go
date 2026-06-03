package build

import (
	"fmt"
	"runtime"
	"time"
)

// Version, Hash, Date, Builder, Branch, and CGO are stamped at build time via ldflags.
var (
	Version string
	Hash    string
	Date    string // build timestamp, UTC (2026-02-17T15:04:05Z)
	Builder string // build environment OS and distro
	Branch  string // git ref at build time
	CGO     string // "1" if cgo was enabled, "0" or "" otherwise
)

// DevVersion is the base version shown for unstamped developer builds. Client
// applications should set this at init time (e.g. "v0.1.x", "v2.0.x").
var DevVersion = "v0.0.x"

// String returns a compact version string. If Version and Hash are not stamped
// (i.e. a local developer build), it returns a fallback string.
func String() string {
	if Version != "" && Hash != "" {
		return fmt.Sprintf("%s [%s]", Version, Hash)
	}
	return DevVersion + " [developer build]"
}

// Detail returns a multi-line block with full build metadata. Stamped fields
// (version, commit, built, branch, builder) only appear when set via ldflags.
// Runtime fields (go, target) always appear.
func Detail() string {
	var out string
	if Version != "" && Hash != "" {
		out += fmt.Sprintf("version:  %s\n", Version)
		out += fmt.Sprintf("commit:   %s\n", Hash)
		if Date != "" {
			out += fmt.Sprintf("built:    %s\n", formatBuildTime(Date))
		}
		if Branch != "" {
			out += fmt.Sprintf("branch:   %s\n", Branch)
		}
		if Builder != "" {
			out += fmt.Sprintf("builder:  %s\n", Builder)
		}
	} else {
		out += fmt.Sprintf("version:  %s\n", String())
	}
	goLine := runtime.Version()
	if CGO == "1" {
		goLine += " (cgo)"
	}
	out += fmt.Sprintf("go:       %s\n", goLine)
	out += fmt.Sprintf("target:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return out
}

// formatBuildTime parses an RFC3339 build timestamp, converts it to local time,
// and appends a human-readable age. Falls back to the raw string on parse failure.
func formatBuildTime(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	local := t.Local().Format("2006-01-02 15:04:05 -0700")
	return fmt.Sprintf("%s (%s)", local, formatAge(time.Since(t)))
}

// formatAge formats a duration as a human-readable age string using at most two
// significant units.
func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	const (
		day  = 24 * time.Hour
		hour = time.Hour
		min  = time.Minute
	)

	switch {
	case d < min:
		return "just now"
	case d < hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < day:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm ago", h, m)
	case d < 30*day:
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh ago", days, hours)
	default:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd ago", days)
	}
}
