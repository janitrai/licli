package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newLinkedIn(cfg)
		if err != nil {
			return err
		}

		text := strings.Join(args, " ")
		me, _ := li.GetMe(context.Background())

		res, err := li.CreatePost(context.Background(), me.MemberURN, text)
		if err != nil {
			return err
		}
		if res.EntityURN != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Posted: %s\n", res.EntityURN)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Posted.")
		}
		return nil
	},
}

var postListLimit int

var postListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent posts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newLinkedIn(cfg)
		if err != nil {
			return err
		}

		me, err := li.GetMe(context.Background())
		if err != nil {
			return err
		}

		updates, err := li.ListProfilePosts(context.Background(), me.PublicIdentifier, 0, postListLimit)
		if err != nil {
			return err
		}

		for _, u := range updates {
			ts := ""
			if u.PublishedAt > 0 {
				// LinkedIn typically uses ms since epoch for these fields.
				t := time.UnixMilli(u.PublishedAt).UTC()
				ts = t.Format(time.RFC3339)
			}
			line := u.Commentary
			line = strings.ReplaceAll(line, "\n", " ")
			line = strings.TrimSpace(line)
			if len(line) > 120 {
				line = line[:120] + "..."
			}

			if ts != "" {
				if line != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", ts, u.EntityURN, line)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", ts, u.EntityURN)
				}
				continue
			}

			if line != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", u.EntityURN, line)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), u.EntityURN)
			}
		}
		return nil
	},
}

func init() {
	postCmd.AddCommand(postCreateCmd)
	postCmd.AddCommand(postListCmd)

	postListCmd.Flags().IntVar(&postListLimit, "limit", 10, "Max posts to show")
}
