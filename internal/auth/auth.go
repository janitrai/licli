package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type Cookies struct {
	LiAt       string
	JSessionID string
}

func (c Cookies) Valid() bool {
	return c.LiAt != "" && c.JSessionID != ""
}

func (c Cookies) CSRFToken() string {
	return strings.Trim(c.JSessionID, "\"")
}

func (c Cookies) JSessionIDCookieValue() string {
	if c.JSessionID == "" {
		return ""
	}
	if strings.HasPrefix(c.JSessionID, "\"") && strings.HasSuffix(c.JSessionID, "\"") {
		return c.JSessionID
	}
	// LinkedIn typically stores JSESSIONID quoted; ensure the cookie header matches that format.
	return fmt.Sprintf("%q", c.JSessionID)
}

func (c Cookies) CookieHeader() string {
	var b strings.Builder
	if c.LiAt != "" {
		fmt.Fprintf(&b, "li_at=%s", c.LiAt)
	}
	if v := c.JSessionIDCookieValue(); v != "" {
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "JSESSIONID=%s", v)
	}
	return b.String()
}

// NormalizePublicIdentifier accepts inputs like "@jane-doe", "jane-doe",
// "linkedin.com/in/jane-doe/", or "https://www.linkedin.com/in/jane-doe/".
func NormalizePublicIdentifier(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	s = strings.Trim(s, "/")

	if strings.Contains(s, "linkedin.com/") {
		if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
			s = "https://" + s
		}
		u, err := url.Parse(s)
		if err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			for i := 0; i < len(parts)-1; i++ {
				switch parts[i] {
				case "in", "pub":
					return parts[i+1]
				}
			}
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}

	return s
}

func OpenBrowser(rawURL string) error {
	if rawURL == "" {
		return errors.New("empty url")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

type ChromeLoginOptions struct {
	Timeout  time.Duration
	Headless bool
	LoginURL string
}

func LoginWithChrome(ctx context.Context, opts ChromeLoginOptions) (Cookies, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	if opts.LoginURL == "" {
		opts.LoginURL = "https://www.linkedin.com/login"
	}

	userDataDir, err := os.MkdirTemp("", "li-chrome-*")
	if err != nil {
		return Cookies{}, fmt.Errorf("create temp chrome profile: %w", err)
	}
	defer func() { _ = os.RemoveAll(userDataDir) }()

	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocOpts = append(allocOpts,
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("headless", opts.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	if runtime.GOOS == "linux" {
		// Commonly required in container-like environments.
		allocOpts = append(allocOpts, chromedp.Flag("no-sandbox", true))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()

	bctx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	timeoutCtx, cancelTimeout := context.WithTimeout(bctx, opts.Timeout)
	defer cancelTimeout()

	if err := chromedp.Run(timeoutCtx,
		network.Enable(),
		chromedp.Navigate(opts.LoginURL),
	); err != nil {
		return Cookies{}, fmt.Errorf("launch chrome: %w", err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		var out Cookies
		cookies, err := network.GetCookies().WithUrls([]string{"https://www.linkedin.com/"}).Do(timeoutCtx)
		if err == nil {
			for _, ck := range cookies {
				switch ck.Name {
				case "li_at":
					out.LiAt = ck.Value
				case "JSESSIONID":
					out.JSessionID = ck.Value
				}
			}
			if out.Valid() {
				return out, nil
			}
		}

		select {
		case <-ticker.C:
			continue
		case <-timeoutCtx.Done():
			return Cookies{}, fmt.Errorf("timed out waiting for LinkedIn cookies (li_at, JSESSIONID)")
		}
	}
}
