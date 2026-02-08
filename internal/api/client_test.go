package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/janitrai/bragcli/internal/auth"
)

func TestClientDo_SetsAuthHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/voyager/api/me" {
			t.Fatalf("path = %s, want %s", r.URL.Path, "/voyager/api/me")
		}

		if got := r.Header.Get("csrf-token"); got != "ajax:123" {
			t.Fatalf("csrf-token = %q, want %q", got, "ajax:123")
		}
		cookie := r.Header.Get("cookie")
		if !strings.Contains(cookie, "li_at=liat") {
			t.Fatalf("cookie missing li_at, got: %q", cookie)
		}
		if !strings.Contains(cookie, `JSESSIONID="ajax:123"`) {
			t.Fatalf("cookie missing JSESSIONID, got: %q", cookie)
		}

		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer ts.Close()

	cookies := auth.Cookies{LiAt: "liat", JSessionID: "ajax:123"}
	c, err := NewClient(cookies, WithBaseURL(ts.URL+"/voyager/api"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var out map[string]any
	if err := c.Do(context.Background(), http.MethodGet, "/me", nil, nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
}
