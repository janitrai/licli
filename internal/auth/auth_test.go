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

func TestCSRFToken_StripsQuotes(t *testing.T) {
	tests := []struct {
		name       string
		jsessionid string
		want       string
	}{
		{"unquoted", "ajax:123", "ajax:123"},
		{"quoted", `"ajax:123"`, "ajax:123"},
		{"double-quoted", `""ajax:123""`, "ajax:123"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Cookies{JSessionID: tt.jsessionid}
			if got := c.CSRFToken(); got != tt.want {
				t.Errorf("CSRFToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCookieHeader_JSessionIDQuoting(t *testing.T) {
	tests := []struct {
		name       string
		jsessionid string
		wantInHdr  string
	}{
		{
			"unquoted gets quoted",
			"ajax:456",
			`JSESSIONID="ajax:456"`,
		},
		{
			"already quoted stays quoted",
			`"ajax:456"`,
			`JSESSIONID="ajax:456"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Cookies{LiAt: "tok", JSessionID: tt.jsessionid}
			got := c.CookieHeader()
			if got == "" {
				t.Fatal("CookieHeader() is empty")
			}
			// Check the JSESSIONID portion
			if !contains(got, tt.wantInHdr) {
				t.Errorf("CookieHeader() = %q, want to contain %q", got, tt.wantInHdr)
			}
		})
	}
}

func TestCookieHeader_OnlyLiAt(t *testing.T) {
	c := Cookies{LiAt: "tok"}
	got := c.CookieHeader()
	if got != "li_at=tok" {
		t.Errorf("CookieHeader() = %q, want %q", got, "li_at=tok")
	}
}

func TestCookieHeader_OnlyJSession(t *testing.T) {
	c := Cookies{JSessionID: "ajax:x"}
	got := c.CookieHeader()
	if got != `JSESSIONID="ajax:x"` {
		t.Errorf("CookieHeader() = %q, want %q", got, `JSESSIONID="ajax:x"`)
	}
}

func TestCookieHeader_Empty(t *testing.T) {
	c := Cookies{}
	if got := c.CookieHeader(); got != "" {
		t.Errorf("CookieHeader() = %q, want empty", got)
	}
}

func TestCookies_Valid(t *testing.T) {
	tests := []struct {
		name string
		c    Cookies
		want bool
	}{
		{"both present", Cookies{LiAt: "a", JSessionID: "b"}, true},
		{"missing LiAt", Cookies{JSessionID: "b"}, false},
		{"missing JSessionID", Cookies{LiAt: "a"}, false},
		{"both empty", Cookies{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePublicIdentifier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"with @", "@jane-doe", "jane-doe"},
		{"plain", "jane-doe", "jane-doe"},
		{"full URL /in/", "https://www.linkedin.com/in/jane-doe/", "jane-doe"},
		{"URL without scheme", "linkedin.com/in/jane-doe", "jane-doe"},
		{"URL with /pub/ prefix", "https://www.linkedin.com/pub/jane-doe/12/34/567", "jane-doe"},
		{"URL with query params", "https://www.linkedin.com/in/john-smith?trk=org-employees", "john-smith"},
		{"URL with fragment", "https://www.linkedin.com/in/john-smith#experience", "john-smith"},
		{"URL with query and fragment", "https://www.linkedin.com/in/jane-doe/?param=1#top", "jane-doe"},
		{"empty string", "", ""},
		{"just @", "@", ""},
		{"profile with dots", "john.smith", "john.smith"},
		{"profile with hyphens", "mary-jane-watson", "mary-jane-watson"},
		{"profile with dots and hyphens", "j.r-smith", "j.r-smith"},
		{"profile with numbers", "user123", "user123"},
		{"trailing slash", "jane-doe/", "jane-doe"},
		{"leading/trailing spaces", "  jane-doe  ", "jane-doe"},
		{"http URL", "http://linkedin.com/in/jane-doe", "jane-doe"},
		{"www URL without /in/", "https://www.linkedin.com/some/path", "path"},
		{"URL with trailing slash", "https://www.linkedin.com/in/test-user/", "test-user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePublicIdentifier(tt.in); got != tt.want {
				t.Errorf("NormalizePublicIdentifier(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// contains is a helper since strings.Contains isn't imported
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
