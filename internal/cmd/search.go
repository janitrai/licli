package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search LinkedIn",
}

var searchPeopleCmd = &cobra.Command{
	Use:   "people [query]",
	Short: "Search for people",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("TODO: Search people: %s\n", args[0])
		return nil
	},
}

var searchJobsCmd = &cobra.Command{
	Use:   "jobs [query]",
	Short: "Search for jobs",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("TODO: Search jobs: %s\n", args[0])
		return nil
	},
}

func init() {
	searchCmd.AddCommand(searchPeopleCmd)
	searchCmd.AddCommand(searchJobsCmd)
}
