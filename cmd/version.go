package cmd

import "fmt"

// VersionCmd represents the version command structure
type VersionCmd struct{}

// Run implements the version command execution
func (v *VersionCmd) Run(cli *CLI) error {
	fmt.Printf("ghub-desk version %s\n", appVersion)
	fmt.Printf("commit: %s\n", appCommit)
	fmt.Printf("built at: %s\n", appDate)
	return nil
}
