package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/horsefit/li/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with LinkedIn",
}

var (
	authManual   bool
	authHeadless bool
	authTimeout  time.Duration
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to LinkedIn",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := loadConfig()
		if err != nil {
			return err
		}

		var cookies auth.Cookies
		if authManual {
			_ = auth.OpenBrowser("https://www.linkedin.com/login")
			fmt.Fprintln(cmd.ErrOrStderr(), "Paste your LinkedIn cookies (from browser devtools -> Application/Storage -> Cookies -> https://www.linkedin.com).")

			r := bufio.NewReader(cmd.InOrStdin())
			fmt.Fprint(cmd.ErrOrStderr(), "li_at: ")
			liAt, err := r.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read li_at: %w", err)
			}
			fmt.Fprint(cmd.ErrOrStderr(), "JSESSIONID: ")
			jsid, err := r.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read JSESSIONID: %w", err)
			}
			cookies = auth.Cookies{
				LiAt:       strings.TrimSpace(liAt),
				JSessionID: strings.TrimSpace(jsid),
			}
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), "A Chrome window will open. Complete LinkedIn login, then return to this terminal.")
			ctx := context.Background()
			cookies, err = auth.LoginWithChrome(ctx, auth.ChromeLoginOptions{
				Timeout:  authTimeout,
				Headless: authHeadless,
				LoginURL: "https://www.linkedin.com/login",
			})
			if err != nil {
				return fmt.Errorf("browser login failed: %w (try --manual)", err)
			}
		}

		if !cookies.Valid() {
			return fmt.Errorf("did not capture required cookies (li_at, JSESSIONID)")
		}

		cfg.Auth.LiAt = cookies.LiAt
		cfg.Auth.JSessionID = cookies.JSessionID
		if err := saveConfig(path, cfg); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Logged in. Saved auth to %s\n", path)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := loadConfig()
		if err != nil {
			return err
		}

		if !cfg.Auth.LoggedIn() {
			fmt.Fprintf(cmd.OutOrStdout(), "Not logged in. Config: %s\n", path)
			return nil
		}

		li, err := newLinkedIn(cfg)
		if err != nil {
			// Cookies exist but can't build a client for some reason.
			fmt.Fprintf(cmd.OutOrStdout(), "Auth present but unusable: %v\nConfig: %s\n", err, path)
			return nil
		}

		me, err := li.GetMe(context.Background())
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Auth present but request failed: %v\nConfig: %s\n", err, path)
			return nil
		}

		name := strings.TrimSpace(strings.TrimSpace(me.FirstName + " " + me.LastName))
		if name == "" {
			name = "unknown"
		}
		if me.PublicIdentifier != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s (%s). Config: %s\n", name, me.PublicIdentifier, path)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s. Config: %s\n", name, path)
		}
		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)

	authLoginCmd.Flags().BoolVar(&authManual, "manual", false, "Manually paste cookies instead of using a controlled Chrome session")
	authLoginCmd.Flags().BoolVar(&authHeadless, "headless", false, "Run Chrome in headless mode (usually requires pre-existing login state)")
	authLoginCmd.Flags().DurationVar(&authTimeout, "timeout", 10*time.Minute, "How long to wait for you to complete login in the browser")
}
