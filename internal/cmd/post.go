package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var postCmd = &cobra.Command{
	Use:   "post",
	Short: "Manage LinkedIn posts",
}

var postCreateCmd = &cobra.Command{
	Use:   "create [text]",
	Short: "Create a new post",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("TODO: Create post: %s\n", args[0])
		return nil
	},
}

var postListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent posts",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TODO: List posts")
		return nil
	},
}

func init() {
	postCmd.AddCommand(postCreateCmd)
	postCmd.AddCommand(postListCmd)
}
