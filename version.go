// Package main provides version information for the Aeron Toolbox API.
package main

// Version is the build version string, set at build time via ldflags.
// It defaults to "dev" for development builds.
var Version = "dev"

// Commit is the git commit hash, set at build time via ldflags.
// It defaults to "unknown" when not built from version control.
var Commit = "unknown"

// BuildTime is the build timestamp, set at build time via ldflags.
// It defaults to "unknown" when build time is not captured.
var BuildTime = "unknown"
