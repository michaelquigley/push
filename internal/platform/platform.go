package platform

import "runtime"

// Detect returns the platform string for the current host (e.g. "linux-amd64").
func Detect() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}
