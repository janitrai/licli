package cmd

import (
	"context"
	"fmt"

	"github.com/horsefit/li/internal/auth"
	"github.com/spf13/cobra"
)

var connectNote string

var connectCmd = &cobra.Command{
	Use:   "connect [username]",
	Short: "Send a connection request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newLinkedIn(cfg)
		if err != nil {
			return err
		}

		publicID := auth.NormalizePublicIdentifier(args[0])
		profile, err := li.GetProfile(context.Background(), publicID)
		if err != nil {
			return err
		}
		if profile.MemberID == "" {
			return fmt.Errorf("could not determine profile id for %q", publicID)
		}

		if err := li.Connect(context.Background(), profile.MemberID, connectNote); err != nil {
			return err
		}

		if connectNote != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Sent connection request to %s (with note)\n", publicID)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Sent connection request to %s\n", publicID)
		}
		return nil
	},
}

func init() {
	connectCmd.Flags().StringVar(&connectNote, "note", "", "Add a note to the connection request")
}
