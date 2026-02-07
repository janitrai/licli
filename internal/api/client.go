package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/horsefit/li/internal/auth"
)

const (
	DefaultBaseURL = "https://www.linkedin.com/voyager/api"

	defaultUserAgent      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"
	defaultAcceptLanguage = "en-US,en;q=0.9"
)

type Client struct {
	BaseURL *url.URL
	HTTP    *http.Client

	Cookies auth.Cookies

	UserAgent string
	Debug     bool
	DebugOut  io.Writer
}

type Option func(*Client) error

func WithBaseURL(raw string) Option {
	return func(c *Client) error {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("parse base url: %w", err)
		}
		c.BaseURL = u
		return nil
	}
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) error {
		if h != nil {
			c.HTTP = h
		}
		return nil
	}
}

func WithDebug(out io.Writer) Option {
	return func(c *Client) error {
		c.Debug = true
		if out != nil {
			c.DebugOut = out
		}
		return nil
	}
}

func NewClient(cookies auth.Cookies, opts ...Option) (*Client, error) {
	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse default base url: %w", err)
	}

	c := &Client{
		BaseURL: u,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
		Cookies:   cookies,
		UserAgent: defaultUserAgent,
		DebugOut:  io.Discard,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.BaseURL == nil {
		return nil, fmt.Errorf("base url is nil")
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	return c, nil
}

type HTTPError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.URL, e.StatusCode)
	}
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

// DoRaw is like Do but accepts a pre-built raw query string (not url.Values)
// to avoid double-encoding LinkedIn's tuple syntax.
func (c *Client) DoRaw(ctx context.Context, method, path string, rawQuery string, body any, out any) error {
	return c.doInternal(ctx, method, path, rawQuery, body, out, nil)
}

// DoMessaging is like DoRaw but overrides Content-Type and Accept headers
// for LinkedIn's messaging write endpoints (which require text/plain to
// avoid CORS preflight and return plain JSON).
func (c *Client) DoMessaging(ctx context.Context, method, path string, rawQuery string, body any, out any) error {
	overrides := map[string]string{
		"content-type": "text/plain;charset=UTF-8",
		"accept":       "application/json",
	}
	return c.doInternal(ctx, method, path, rawQuery, body, out, overrides)
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var rawQuery string
	if query != nil {
		rawQuery = query.Encode()
	}
	return c.doInternal(ctx, method, path, rawQuery, body, out, nil)
}

func (c *Client) doInternal(ctx context.Context, method, path string, rawQuery string, body any, out any, headerOverrides map[string]string) error {
	if c.Cookies.LiAt == "" || c.Cookies.JSessionID == "" {
		return fmt.Errorf("missing auth cookies (li_at, JSESSIONID)")
	}

	u := *c.BaseURL
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	if rawQuery != "" {
		u.RawQuery = rawQuery
	}

	var bodyReader io.Reader
	var contentType string
	if body != nil {
		switch v := body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("marshal request json: %w", err)
			}
			bodyReader = bytes.NewReader(b)
			contentType = "application/json; charset=utf-8"
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("user-agent", c.UserAgent)
	req.Header.Set("accept", "application/vnd.linkedin.normalized+json+2.1")
	req.Header.Set("accept-language", defaultAcceptLanguage)
	req.Header.Set("x-li-lang", "en_US")
	req.Header.Set("x-restli-protocol-version", "2.0.0")
	req.Header.Set("csrf-token", c.Cookies.CSRFToken())
	req.Header.Set("cookie", c.Cookies.CookieHeader())
	if contentType != "" && req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", contentType)
	}
	// Apply any per-request header overrides (e.g. messaging endpoints).
	for k, v := range headerOverrides {
		req.Header.Set(k, v)
	}

	if c.Debug {
		fmt.Fprintf(c.DebugOut, "[li] %s %s\n", method, u.String())
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	const maxBody = 5 << 20 // 5 MiB
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))

	if c.Debug {
		fmt.Fprintf(c.DebugOut, "[li] -> %d (%d bytes)\n", resp.StatusCode, len(respBody))
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return &HTTPError{
			Method:     method,
			URL:        u.String(),
			StatusCode: resp.StatusCode,
			Body:       "rate limited by LinkedIn, try again later",
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 2000 {
			snippet = snippet[:2000] + "…"
		}
		return &HTTPError{
			Method:     method,
			URL:        u.String(),
			StatusCode: resp.StatusCode,
			Body:       snippet,
		}
	}

	if out == nil {
		return nil
	}
	if len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response json: %w", err)
	}
	return nil
}
