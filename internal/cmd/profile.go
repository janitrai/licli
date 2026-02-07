package cmd

import (
	"fmt"
	"strings"

	"github.com/horsefit/li/internal/auth"
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
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newLinkedIn(cfg)
		if err != nil {
			return err
		}

		publicID := ""
		if len(args) == 0 {
			me, err := li.GetMe(cmd.Context())
			if err != nil {
				return err
			}
			publicID = me.PublicIdentifier
		} else {
			publicID = auth.NormalizePublicIdentifier(args[0])
		}
		if strings.TrimSpace(publicID) == "" {
			return fmt.Errorf("missing profile identifier")
		}

		p, err := li.GetProfile(cmd.Context(), publicID)
		if err != nil {
			return err
		}

		name := strings.TrimSpace(p.FirstName + " " + p.LastName)
		if name == "" {
			name = publicID
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", name)
		if p.Headline != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Headline: %s\n", p.Headline)
		}
		if p.LocationName != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Location: %s\n", p.LocationName)
		}
		if p.PublicIdentifier != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Public ID: %s\n", p.PublicIdentifier)
		}
		if p.MemberURN != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Member URN: %s\n", p.MemberURN)
		}
		if p.Summary != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", p.Summary)
		}
		return nil
	},
}

var profileMeCmd = &cobra.Command{
	Use:   "me",
	Short: "View your own profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return profileViewCmd.RunE(cmd, args)
	},
}

func init() {
	profileCmd.AddCommand(profileViewCmd)
	profileCmd.AddCommand(profileMeCmd)
}
