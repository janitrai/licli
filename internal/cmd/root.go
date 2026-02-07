package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "li",
	Short: "LinkedIn CLI",
	Long:  `li is a command-line interface for LinkedIn, inspired by gh (GitHub CLI).`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add subcommands here
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(postCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(followCmd)
}
