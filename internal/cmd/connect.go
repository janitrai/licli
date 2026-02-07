package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var connectNote string

var connectCmd = &cobra.Command{
	Use:   "connect [username]",
	Short: "Send a connection request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if connectNote != "" {
			fmt.Printf("TODO: Connect to %s with note: %s\n", args[0], connectNote)
		} else {
			fmt.Printf("TODO: Connect to %s\n", args[0])
		}
		return nil
	},
}

func init() {
	connectCmd.Flags().StringVar(&connectNote, "note", "", "Add a note to the connection request")
}
