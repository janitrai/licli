package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var followCmd = &cobra.Command{
	Use:   "follow [username]",
	Short: "Follow a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("TODO: Follow %s\n", args[0])
		return nil
	},
}
