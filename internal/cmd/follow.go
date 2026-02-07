package cmd

import (
	"fmt"

	"github.com/horsefit/li/internal/auth"
	"github.com/spf13/cobra"
)

var followCmd = &cobra.Command{
	Use:   "follow [username]",
	Short: "Follow a user",
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
		profile, err := li.GetProfile(cmd.Context(), publicID)
		if err != nil {
			return err
		}
		if profile.MemberURN == "" {
			return fmt.Errorf("could not determine member urn for %q", publicID)
		}

		if err := li.Follow(cmd.Context(), profile.MemberURN); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Followed %s\n", publicID)
		return nil
	},
}
