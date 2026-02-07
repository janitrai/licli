package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with LinkedIn",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to LinkedIn",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TODO: Implement LinkedIn login")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TODO: Show auth status")
		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
}
