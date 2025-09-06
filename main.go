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
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
