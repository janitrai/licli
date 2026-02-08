package cmd

import (
	"github.com/spf13/cobra"
)

var (
	cfgPath string
	debug   bool
)

var rootCmd = &cobra.Command{
	Use:   "bragcli",
	Short: "Bragnet CLI",
	Long:  `bragcli is a command-line interface for Bragnet, inspired by gh (GitHub CLI).`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "Path to config file (default: $XDG_CONFIG_HOME/li/config.json)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging (prints HTTP method/url/status)")

	// Add subcommands here
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(postCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(followCmd)
	rootCmd.AddCommand(messageCmd)
}
