// Package main implements a CLI tool for GitHub organization management.
// This tool allows users to fetch, view, and manage GitHub organization data
// including users, teams, repositories, and team memberships.
package main

import (
	"fmt"
	"os"

	"ghub-desk/cmd"
)

func main() {
	// Set version information in cmd package
	cmd.SetVersionInfo(Version, Commit, Date)

	errWriter, cleanup, err := cmd.Execute()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		fmt.Fprintf(errWriter, "Error: %v\n", err)
		os.Exit(1)
	}
}
