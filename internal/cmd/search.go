package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search LinkedIn",
}

var searchLimit int

var searchPeopleCmd = &cobra.Command{
	Use:   "people [query]",
	Short: "Search for people",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newLinkedIn(cfg)
		if err != nil {
			return err
		}

		query := strings.Join(args, " ")
		items, err := li.SearchPeople(context.Background(), query, 0, searchLimit)
		if err != nil {
			return err
		}

		for _, it := range items {
			line := it.PublicIdentifier
			if it.Title != "" {
				if line != "" {
					line += "\t"
				}
				line += it.Title
			}
			if it.PrimarySubtitle != "" {
				line += "\t" + it.PrimarySubtitle
			}
			if it.TargetURN != "" {
				line += "\t" + it.TargetURN
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(line))
		}
		return nil
	},
}

var searchJobsCmd = &cobra.Command{
	Use:   "jobs [query]",
	Short: "Search for jobs",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newLinkedIn(cfg)
		if err != nil {
			return err
		}

		query := strings.Join(args, " ")
		items, err := li.SearchJobs(context.Background(), query, 0, searchLimit)
		if err != nil {
			return err
		}

		for _, it := range items {
			line := it.Title
			if it.PrimarySubtitle != "" {
				line += "\t" + it.PrimarySubtitle
			}
			if it.SecondarySubtitle != "" {
				line += "\t" + it.SecondarySubtitle
			}
			if it.TargetURN != "" {
				line += "\t" + it.TargetURN
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(line))
		}
		return nil
	},
}

func init() {
	searchCmd.AddCommand(searchPeopleCmd)
	searchCmd.AddCommand(searchJobsCmd)

	searchPeopleCmd.Flags().IntVar(&searchLimit, "limit", 10, "Max results to show")
	searchJobsCmd.Flags().IntVar(&searchLimit, "limit", 10, "Max results to show")
}
