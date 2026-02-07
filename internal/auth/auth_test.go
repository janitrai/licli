package auth

import "testing"

func TestCookies_CSRFTokenAndCookieHeader(t *testing.T) {
	c := Cookies{
		LiAt:       "liat",
		JSessionID: "ajax:123",
	}
	if got := c.CSRFToken(); got != "ajax:123" {
		t.Fatalf("CSRFToken() = %q, want %q", got, "ajax:123")
	}
	if got := c.CookieHeader(); got != `li_at=liat; JSESSIONID="ajax:123"` {
		t.Fatalf("CookieHeader() = %q, want %q", got, `li_at=liat; JSESSIONID="ajax:123"`)
	}
}

func TestNormalizePublicIdentifier(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"@jane-doe", "jane-doe"},
		{"jane-doe", "jane-doe"},
		{"https://www.linkedin.com/in/jane-doe/", "jane-doe"},
		{"linkedin.com/in/jane-doe", "jane-doe"},
		{"https://www.linkedin.com/pub/jane-doe/12/34/567", "jane-doe"},
	}
	for _, tt := range tests {
		if got := NormalizePublicIdentifier(tt.in); got != tt.want {
			t.Fatalf("NormalizePublicIdentifier(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
