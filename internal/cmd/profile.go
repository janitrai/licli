package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "View LinkedIn profiles",
}

var profileViewCmd = &cobra.Command{
	Use:   "view [username]",
	Short: "View a profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Println("TODO: View own profile")
		} else {
			fmt.Printf("TODO: View profile: %s\n", args[0])
		}
		return nil
	},
}

var profileMeCmd = &cobra.Command{
	Use:   "me",
	Short: "View your own profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TODO: View own profile")
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileViewCmd)
	profileCmd.AddCommand(profileMeCmd)
}
