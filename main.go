// Package main implements a CLI tool for GitHub organization management.
// This tool allows users to fetch, view, and manage GitHub organization data
// including users, teams, repositories, and team memberships.
package main

import (
	"fmt"
	"os"

	"ghub-desk/cmd"
)

var (
	// version is set during build via ldflags
	version = "dev"
	// commit is set during build via ldflags
	commit = "none"
	// date is set during build via ldflags
	date = "unknown"
)

func main() {
	// Set version information in cmd package
	cmd.SetVersionInfo(version, commit, date)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
