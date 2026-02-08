package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/janitrai/bragcli/internal/api"
	"github.com/janitrai/bragcli/internal/auth"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:     "message",
	Aliases: []string{"msg"},
	Short:   "Bragnet messaging",
}

var messageListLimit int

var messageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent conversations (inbox)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newBragnet(cfg)
		if err != nil {
			return err
		}

		profileURN, err := resolveMyProfileURN(cmd, li)
		if err != nil {
			return err
		}

		convos, err := li.ListConversations(cmd.Context(), profileURN, messageListLimit)
		if err != nil {
			return err
		}

		if len(convos) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No conversations found.")
			return nil
		}

		for _, c := range convos {
			// Build participant names (skip "Me" / self by checking profileURN).
			var names []string
			for _, p := range c.Participants {
				if p.ProfileURN == profileURN {
					continue
				}
				name := p.FullName()
				if name == "" {
					name = p.ProfileURN
				}
				names = append(names, name)
			}
			if len(names) == 0 {
				names = append(names, "(unknown)")
			}

			who := strings.Join(names, ", ")

			if c.LastMessage != nil {
				ts := formatTimestamp(c.LastMessage.DeliveredAt)
				preview := truncate(c.LastMessage.BodyText, 80)
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n  %s\n\n", who, ts, preview)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  (no messages)\n\n", who)
			}
		}
		return nil
	},
}

var messageReadCmd = &cobra.Command{
	Use:   "read <username>",
	Short: "Read messages in a conversation with a specific user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newBragnet(cfg)
		if err != nil {
			return err
		}

		// 1. Resolve my own profile URN.
		myProfileURN, err := resolveMyProfileURN(cmd, li)
		if err != nil {
			return err
		}

		// 2. Resolve target user's profile URN.
		username := auth.NormalizePublicIdentifier(args[0])
		targetProfile, err := li.GetProfile(cmd.Context(), username)
		if err != nil {
			return fmt.Errorf("resolve profile %q: %w", username, err)
		}
		targetURN := targetProfile.MiniProfileEntityURN
		if targetURN == "" {
			return fmt.Errorf("could not determine profile URN for %q", username)
		}

		// 3. List conversations and find the one with the target.
		convos, err := li.ListConversations(cmd.Context(), myProfileURN, 25)
		if err != nil {
			return fmt.Errorf("list conversations: %w", err)
		}

		convo := api.FindConversationByProfileURN(convos, targetURN)
		if convo == nil {
			return fmt.Errorf("no conversation found with %s (%s)", username, targetURN)
		}

		// 4. Fetch messages.
		msgs, err := li.GetMessages(cmd.Context(), convo.EntityURN, 0)
		if err != nil {
			return fmt.Errorf("get messages: %w", err)
		}

		if len(msgs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No messages in this conversation.")
			return nil
		}

		targetName := strings.TrimSpace(targetProfile.FirstName + " " + targetProfile.LastName)
		if targetName == "" {
			targetName = username
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Conversation with %s\n%s\n\n",
			targetName, strings.Repeat("─", 40))

		for _, msg := range msgs {
			sender := msg.SenderName
			if sender == "" {
				sender = msg.SenderURN
			}
			ts := formatTimestamp(msg.DeliveredAt)
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s:\n%s\n\n", ts, sender, msg.BodyText)
		}
		return nil
	},
}

var messageSendCmd = &cobra.Command{
	Use:   "send <username> <message>",
	Short: "Send a message to a user (experimental)",
	Long:  "Send a text message to a Bragnet user. Creates a new conversation if one doesn't exist.\nNote: this command is experimental and may not work with all Bragnet API versions.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}
		li, err := newBragnet(cfg)
		if err != nil {
			return err
		}

		// 1. Resolve my own profile URN.
		myProfileURN, err := resolveMyProfileURN(cmd, li)
		if err != nil {
			return err
		}

		// 2. Resolve target user.
		username := auth.NormalizePublicIdentifier(args[0])
		targetProfile, err := li.GetProfile(cmd.Context(), username)
		if err != nil {
			return fmt.Errorf("resolve profile %q: %w", username, err)
		}
		targetURN := targetProfile.MiniProfileEntityURN
		if targetURN == "" {
			return fmt.Errorf("could not determine profile URN for %q", username)
		}

		text := strings.Join(args[1:], " ")

		// 3. Try to find an existing conversation.
		convos, err := li.ListConversations(cmd.Context(), myProfileURN, 25)
		if err != nil {
			return fmt.Errorf("list conversations: %w", err)
		}

		convo := api.FindConversationByProfileURN(convos, targetURN)
		if convo != nil {
			// Send to existing conversation.
			if err := li.SendMessage(cmd.Context(), myProfileURN, convo.EntityURN, text); err != nil {
				return fmt.Errorf("send message: %w", err)
			}
		} else {
			// Create new conversation.
			if err := li.CreateConversationWithMessage(cmd.Context(), myProfileURN, []string{targetURN}, text); err != nil {
				return fmt.Errorf("create conversation: %w", err)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Message sent to %s.\n", username)
		return nil
	},
}

// resolveMyProfileURN fetches the current user's fsd_profile URN.
// It first tries Me.ProfileURN (dashEntityUrn), then falls back to
// fetching the full profile via the public identifier.
func resolveMyProfileURN(cmd *cobra.Command, li *api.Bragnet) (string, error) {
	me, err := li.GetMe(cmd.Context())
	if err != nil {
		return "", fmt.Errorf("get current user: %w", err)
	}

	if me.ProfileURN != "" {
		return me.ProfileURN, nil
	}

	// Fallback: fetch full profile to get fsd_profile URN.
	if me.PublicIdentifier == "" {
		return "", fmt.Errorf("could not determine your profile URN (no publicIdentifier from /me)")
	}
	prof, err := li.GetProfile(cmd.Context(), me.PublicIdentifier)
	if err != nil {
		return "", fmt.Errorf("get own profile: %w", err)
	}
	if prof.MiniProfileEntityURN == "" {
		return "", fmt.Errorf("could not determine your fsd_profile URN")
	}
	return prof.MiniProfileEntityURN, nil
}

func formatTimestamp(ms int64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if now.Sub(t) < 7*24*time.Hour {
		return t.Format("Mon 15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func truncate(s string, max int) string {
	// Replace newlines with spaces for preview.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

func init() {
	messageCmd.AddCommand(messageListCmd)
	messageCmd.AddCommand(messageReadCmd)
	messageCmd.AddCommand(messageSendCmd)

	messageListCmd.Flags().IntVar(&messageListLimit, "limit", 20, "Max conversations to show")
}
