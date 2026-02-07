package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/horsefit/li/internal/auth"
)

// ---------------------------------------------------------------------------
// urnID
// ---------------------------------------------------------------------------

func TestUrnID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"member URN", "urn:li:member:12345", "12345"},
		{"miniProfile URN", "urn:li:fs_miniProfile:ACoAAAIBCD", "ACoAAAIBCD"},
		{"fsd_profile URN", "urn:li:fsd_profile:ACoAAAIBCD", "ACoAAAIBCD"},
		{"activity URN", "urn:li:activity:7000000000000000000", "7000000000000000000"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"no colon", "nocolon", ""},
		{"trailing colon", "urn:li:member:", ""},
		{"single segment with colon", "foo:bar", "bar"},
		{"leading/trailing spaces", "  urn:li:member:999  ", "999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urnID(tt.in)
			if got != tt.want {
				t.Errorf("urnID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getString (variadic path walker)
// ---------------------------------------------------------------------------

func TestGetString(t *testing.T) {
	m := map[string]any{
		"a": "hello",
		"b": map[string]any{
			"c": "nested",
			"d": map[string]any{
				"e": "deep",
			},
		},
		"num":  42,
		"null": nil,
	}

	tests := []struct {
		name string
		path []string
		want string
	}{
		{"top-level string", []string{"a"}, "hello"},
		{"nested one level", []string{"b", "c"}, "nested"},
		{"nested two levels", []string{"b", "d", "e"}, "deep"},
		{"missing key", []string{"z"}, ""},
		{"missing nested key", []string{"b", "z"}, ""},
		{"non-string value", []string{"num"}, ""},
		{"null value", []string{"null"}, ""},
		{"wrong intermediate type", []string{"a", "b"}, ""},
		{"empty path", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getString(m, tt.path...)
			if got != tt.want {
				t.Errorf("getString(%v) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}

	t.Run("nil map", func(t *testing.T) {
		got := getString(nil, "a")
		if got != "" {
			t.Errorf("getString(nil, a) = %q, want empty", got)
		}
	})
}

// ---------------------------------------------------------------------------
// getNestedText
// ---------------------------------------------------------------------------

func TestGetNestedText(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{
			"plain string",
			map[string]any{"title": "Software Engineer"},
			"title",
			"Software Engineer",
		},
		{
			"object with text field",
			map[string]any{"title": map[string]any{"text": "Software Engineer"}},
			"title",
			"Software Engineer",
		},
		{
			"missing key",
			map[string]any{"other": "val"},
			"title",
			"",
		},
		{
			"object without text",
			map[string]any{"title": map[string]any{"foo": "bar"}},
			"title",
			"",
		},
		{
			"non-string non-map",
			map[string]any{"title": 123},
			"title",
			"",
		},
		{
			"nil value",
			map[string]any{"title": nil},
			"title",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNestedText(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getNestedText(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findMiniProfile
// ---------------------------------------------------------------------------

func TestFindMiniProfile(t *testing.T) {
	t.Run("from included by entityUrn", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"$type":            "com.linkedin.voyager.identity.shared.MiniProfile",
					"entityUrn":        "urn:li:fs_miniProfile:ACoAAB12345",
					"publicIdentifier": "john-doe",
					"firstName":        "John",
					"lastName":         "Doe",
				},
			},
		}
		mp := findMiniProfile(raw)
		if mp == nil {
			t.Fatal("expected non-nil miniProfile")
		}
		if got := getString(mp, "publicIdentifier"); got != "john-doe" {
			t.Errorf("publicIdentifier = %q, want %q", got, "john-doe")
		}
	})

	t.Run("from included by dashEntityUrn fsd_profile", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"dashEntityUrn":    "urn:li:fsd_profile:ACoAAB12345",
					"publicIdentifier": "jane-smith",
					"firstName":        "Jane",
				},
			},
		}
		mp := findMiniProfile(raw)
		if mp == nil {
			t.Fatal("expected non-nil miniProfile")
		}
		if got := getString(mp, "firstName"); got != "Jane" {
			t.Errorf("firstName = %q, want %q", got, "Jane")
		}
	})

	t.Run("from included by $type MiniProfile", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"$type":     "com.linkedin.voyager.identity.shared.MiniProfile",
					"entityUrn": "urn:li:other:123",
					"firstName": "TypeMatch",
				},
			},
		}
		mp := findMiniProfile(raw)
		if mp == nil {
			t.Fatal("expected non-nil miniProfile")
		}
		if got := getString(mp, "firstName"); got != "TypeMatch" {
			t.Errorf("firstName = %q, want %q", got, "TypeMatch")
		}
	})

	t.Run("fallback to data.miniProfile", func(t *testing.T) {
		raw := map[string]any{
			"data": map[string]any{
				"miniProfile": map[string]any{
					"publicIdentifier": "fallback-user",
				},
			},
		}
		mp := findMiniProfile(raw)
		if mp == nil {
			t.Fatal("expected non-nil miniProfile")
		}
		if got := getString(mp, "publicIdentifier"); got != "fallback-user" {
			t.Errorf("publicIdentifier = %q, want %q", got, "fallback-user")
		}
	})

	t.Run("fallback to top-level miniProfile", func(t *testing.T) {
		raw := map[string]any{
			"miniProfile": map[string]any{
				"publicIdentifier": "toplevel-user",
			},
		}
		mp := findMiniProfile(raw)
		if mp == nil {
			t.Fatal("expected non-nil miniProfile")
		}
		if got := getString(mp, "publicIdentifier"); got != "toplevel-user" {
			t.Errorf("publicIdentifier = %q, want %q", got, "toplevel-user")
		}
	})

	t.Run("empty response returns nil", func(t *testing.T) {
		mp := findMiniProfile(map[string]any{})
		if mp != nil {
			t.Errorf("expected nil, got %v", mp)
		}
	})

	t.Run("included with non-profile items skipped", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"$type":     "com.linkedin.voyager.common.Industry",
					"entityUrn": "urn:li:fs_industry:1",
					"name":      "Tech",
				},
				map[string]any{
					"$type":            "com.linkedin.voyager.identity.shared.MiniProfile",
					"entityUrn":        "urn:li:fs_miniProfile:ACoAAB99999",
					"publicIdentifier": "correct-user",
				},
			},
		}
		mp := findMiniProfile(raw)
		if mp == nil {
			t.Fatal("expected non-nil miniProfile")
		}
		if got := getString(mp, "publicIdentifier"); got != "correct-user" {
			t.Errorf("publicIdentifier = %q, want %q", got, "correct-user")
		}
	})
}

// ---------------------------------------------------------------------------
// findProfileInIncluded
// ---------------------------------------------------------------------------

func TestFindProfileInIncluded(t *testing.T) {
	t.Run("finds by $type and fsd_profile URN", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"$type":            "com.linkedin.voyager.dash.identity.profile.Profile",
					"entityUrn":        "urn:li:fsd_profile:ACoAAB12345",
					"publicIdentifier": "jane-doe",
					"firstName":        "Jane",
					"lastName":         "Doe",
					"headline":         "CTO at Acme",
				},
			},
		}
		prof := findProfileInIncluded(raw)
		if prof == nil {
			t.Fatal("expected non-nil profile")
		}
		if got := getString(prof, "publicIdentifier"); got != "jane-doe" {
			t.Errorf("publicIdentifier = %q, want %q", got, "jane-doe")
		}
	})

	t.Run("skips non-Profile types", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"$type":     "com.linkedin.voyager.common.Industry",
					"entityUrn": "urn:li:fs_industry:96",
					"name":      "IT",
				},
				map[string]any{
					"$type":     "com.linkedin.voyager.dash.identity.profile.Profile",
					"entityUrn": "urn:li:fsd_profile:ACoAAAXYZ",
					"firstName": "Bob",
				},
			},
		}
		prof := findProfileInIncluded(raw)
		if prof == nil {
			t.Fatal("expected non-nil profile")
		}
		if got := getString(prof, "firstName"); got != "Bob" {
			t.Errorf("firstName = %q, want %q", got, "Bob")
		}
	})

	t.Run("fallback to firstName heuristic", func(t *testing.T) {
		raw := map[string]any{
			"included": []any{
				map[string]any{
					"$type":     "com.linkedin.voyager.unknown",
					"entityUrn": "urn:li:other:123",
					"firstName": "Heuristic",
				},
			},
		}
		prof := findProfileInIncluded(raw)
		if prof == nil {
			t.Fatal("expected non-nil profile via fallback")
		}
		if got := getString(prof, "firstName"); got != "Heuristic" {
			t.Errorf("firstName = %q, want %q", got, "Heuristic")
		}
	})

	t.Run("empty included returns nil", func(t *testing.T) {
		raw := map[string]any{"included": []any{}}
		if prof := findProfileInIncluded(raw); prof != nil {
			t.Errorf("expected nil, got %v", prof)
		}
	})

	t.Run("no included key returns nil", func(t *testing.T) {
		raw := map[string]any{"data": map[string]any{}}
		if prof := findProfileInIncluded(raw); prof != nil {
			t.Errorf("expected nil, got %v", prof)
		}
	})
}

// ---------------------------------------------------------------------------
// getInt64
// ---------------------------------------------------------------------------

func TestGetInt64(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want int64
	}{
		{"float64", map[string]any{"ts": float64(1706000000000)}, "ts", 1706000000000},
		{"int64", map[string]any{"ts": int64(42)}, "ts", 42},
		{"int", map[string]any{"ts": int(99)}, "ts", 99},
		{"missing", map[string]any{}, "ts", 0},
		{"string type", map[string]any{"ts": "123"}, "ts", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getInt64(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getInt64(%v, %q) = %d, want %d", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findCommentaryText
// ---------------------------------------------------------------------------

func TestFindCommentaryText(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			"direct commentary.text",
			map[string]any{
				"commentary": map[string]any{"text": "Hello world"},
			},
			"Hello world",
		},
		{
			"shareCommentary.text",
			map[string]any{
				"shareCommentary": map[string]any{"text": "Shared post"},
			},
			"Shared post",
		},
		{
			"nested commentary deep",
			map[string]any{
				"content": map[string]any{
					"commentary": map[string]any{"text": "Deep commentary"},
				},
			},
			"Deep commentary",
		},
		{
			"array wrapping",
			[]any{
				map[string]any{
					"commentary": map[string]any{"text": "In array"},
				},
			},
			"In array",
		},
		{
			"no commentary",
			map[string]any{"foo": "bar"},
			"",
		},
		{
			"nil input",
			nil,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCommentaryText(tt.in)
			if got != tt.want {
				t.Errorf("findCommentaryText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findFirstString
// ---------------------------------------------------------------------------

func TestFindFirstString(t *testing.T) {
	tests := []struct {
		name string
		in   any
		key  string
		want string
	}{
		{
			"top-level",
			map[string]any{"entityUrn": "urn:li:share:123"},
			"entityUrn",
			"urn:li:share:123",
		},
		{
			"nested",
			map[string]any{"data": map[string]any{"entityUrn": "urn:li:share:456"}},
			"entityUrn",
			"urn:li:share:456",
		},
		{
			"in array",
			[]any{map[string]any{"entityUrn": "urn:li:share:789"}},
			"entityUrn",
			"urn:li:share:789",
		},
		{
			"not found",
			map[string]any{"other": "val"},
			"entityUrn",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findFirstString(tt.in, tt.key)
			if got != tt.want {
				t.Errorf("findFirstString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetMe (with mock HTTP server)
// ---------------------------------------------------------------------------

// Realistic /me response fixture in LinkedIn normalized format
const getMeFixture = `{
	"data": {
		"*miniProfile": "urn:li:fs_miniProfile:ACoAAB12345"
	},
	"included": [
		{
			"$type": "com.linkedin.voyager.identity.shared.MiniProfile",
			"entityUrn": "urn:li:fs_miniProfile:ACoAAB12345",
			"objectUrn": "urn:li:member:67890",
			"publicIdentifier": "john-doe",
			"firstName": "John",
			"lastName": "Doe",
			"occupation": "Software Engineer at ACME Corp"
		}
	]
}`

func TestGetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voyager/api/me" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.linkedin.normalized+json+2.1")
		_, _ = io.WriteString(w, getMeFixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "test-li-at", JSessionID: "ajax:test"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	li := NewLinkedIn(c)
	me, err := li.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error: %v", err)
	}

	if me.PublicIdentifier != "john-doe" {
		t.Errorf("PublicIdentifier = %q, want %q", me.PublicIdentifier, "john-doe")
	}
	if me.FirstName != "John" {
		t.Errorf("FirstName = %q, want %q", me.FirstName, "John")
	}
	if me.LastName != "Doe" {
		t.Errorf("LastName = %q, want %q", me.LastName, "Doe")
	}
	if me.Occupation != "Software Engineer at ACME Corp" {
		t.Errorf("Occupation = %q", me.Occupation)
	}
	if me.MiniProfileEntityURN != "urn:li:fs_miniProfile:ACoAAB12345" {
		t.Errorf("MiniProfileEntityURN = %q", me.MiniProfileEntityURN)
	}
	// urnID extracts the last colon-segment from entityUrn first
	if me.MemberID != "ACoAAB12345" {
		t.Errorf("MemberID = %q, want %q", me.MemberID, "ACoAAB12345")
	}
	if me.MemberURN != "urn:li:member:ACoAAB12345" {
		t.Errorf("MemberURN = %q", me.MemberURN)
	}
}

func TestGetMe_FallbackMiniProfile(t *testing.T) {
	// Response where miniProfile is directly under data (non-normalized)
	fixture := `{
		"data": {
			"miniProfile": {
				"publicIdentifier": "flat-user",
				"firstName": "Flat",
				"lastName": "User",
				"occupation": "Tester",
				"entityUrn": "urn:li:fs_miniProfile:FLAT001",
				"objectUrn": "urn:li:member:111"
			}
		}
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	me, err := NewLinkedIn(c).GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error: %v", err)
	}
	if me.PublicIdentifier != "flat-user" {
		t.Errorf("PublicIdentifier = %q, want %q", me.PublicIdentifier, "flat-user")
	}
	// urnID extracts from entityUrn first → last segment of fs_miniProfile URN
	if me.MemberID != "FLAT001" {
		t.Errorf("MemberID = %q, want %q", me.MemberID, "FLAT001")
	}
}

// ---------------------------------------------------------------------------
// GetProfile (with mock HTTP server)
// ---------------------------------------------------------------------------

const getProfileFixture = `{
	"data": {},
	"included": [
		{
			"$type": "com.linkedin.voyager.dash.identity.profile.Profile",
			"entityUrn": "urn:li:fsd_profile:ACoAAAXYZ123",
			"objectUrn": "urn:li:member:54321",
			"publicIdentifier": "jane-smith",
			"firstName": "Jane",
			"lastName": "Smith",
			"headline": "VP of Engineering",
			"summary": "Building great teams.",
			"geoLocationName": "San Francisco Bay Area",
			"locationName": "San Francisco, CA"
		},
		{
			"$type": "com.linkedin.voyager.common.Industry",
			"entityUrn": "urn:li:fs_industry:96",
			"name": "Information Technology"
		}
	]
}`

func TestGetProfile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voyager/api/identity/dash/profiles" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if got := r.URL.Query().Get("memberIdentity"); got != "jane-smith" {
			t.Errorf("memberIdentity = %q, want %q", got, "jane-smith")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, getProfileFixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	prof, err := NewLinkedIn(c).GetProfile(context.Background(), "jane-smith")
	if err != nil {
		t.Fatalf("GetProfile() error: %v", err)
	}

	if prof.PublicIdentifier != "jane-smith" {
		t.Errorf("PublicIdentifier = %q", prof.PublicIdentifier)
	}
	if prof.FirstName != "Jane" {
		t.Errorf("FirstName = %q", prof.FirstName)
	}
	if prof.LastName != "Smith" {
		t.Errorf("LastName = %q", prof.LastName)
	}
	if prof.Headline != "VP of Engineering" {
		t.Errorf("Headline = %q", prof.Headline)
	}
	if prof.Summary != "Building great teams." {
		t.Errorf("Summary = %q", prof.Summary)
	}
	if prof.LocationName != "San Francisco Bay Area" {
		t.Errorf("LocationName = %q (geoLocationName preferred)", prof.LocationName)
	}
	// urnID extracts from entityUrn first → last segment of fsd_profile URN
	if prof.MemberID != "ACoAAAXYZ123" {
		t.Errorf("MemberID = %q, want %q", prof.MemberID, "ACoAAAXYZ123")
	}
	if prof.MemberURN != "urn:li:member:ACoAAAXYZ123" {
		t.Errorf("MemberURN = %q", prof.MemberURN)
	}
}

func TestGetProfile_EmptyIdentifier(t *testing.T) {
	c, _ := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
	)
	_, err := NewLinkedIn(c).GetProfile(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty identifier")
	}
}

// ---------------------------------------------------------------------------
// SearchPeople (with mock HTTP server)
// ---------------------------------------------------------------------------

const searchFixture = `{
	"data": {},
	"included": [
		{
			"$type": "com.linkedin.voyager.dash.search.EntityResultViewModel",
			"entityUrn": "urn:li:fsd_profile:ACoAAAA111",
			"title": {"text": "Alice Johnson"},
			"primarySubtitle": {"text": "Software Engineer at Google"},
			"secondarySubtitle": {"text": "San Francisco, CA"},
			"navigationUrl": "https://www.linkedin.com/in/alice-johnson"
		},
		{
			"$type": "com.linkedin.voyager.dash.search.EntityResultViewModel",
			"entityUrn": "urn:li:fsd_profile:ACoAAAA222",
			"title": {"text": "Bob Williams"},
			"primarySubtitle": "Backend Developer",
			"secondarySubtitle": {"text": "New York, NY"},
			"navigationUrl": "https://www.linkedin.com/in/bob-williams/"
		},
		{
			"$type": "com.linkedin.voyager.dash.search.SearchClusterViewModel",
			"entityUrn": "urn:li:fsd_searchCluster:123",
			"title": "People"
		}
	]
}`

func TestSearchPeople(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voyager/api/graphql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, searchFixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	items, err := NewLinkedIn(c).SearchPeople(context.Background(), "engineer", 0, 10)
	if err != nil {
		t.Fatalf("SearchPeople() error: %v", err)
	}

	// Should find 2 EntityResultViewModel items, skip the SearchClusterViewModel
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	alice := items[0]
	if alice.Title != "Alice Johnson" {
		t.Errorf("items[0].Title = %q", alice.Title)
	}
	if alice.PrimarySubtitle != "Software Engineer at Google" {
		t.Errorf("items[0].PrimarySubtitle = %q", alice.PrimarySubtitle)
	}
	if alice.PublicIdentifier != "alice-johnson" {
		t.Errorf("items[0].PublicIdentifier = %q, want %q", alice.PublicIdentifier, "alice-johnson")
	}

	bob := items[1]
	if bob.Title != "Bob Williams" {
		t.Errorf("items[1].Title = %q", bob.Title)
	}
	// primarySubtitle is a plain string here (not {text: ...})
	if bob.PrimarySubtitle != "Backend Developer" {
		t.Errorf("items[1].PrimarySubtitle = %q", bob.PrimarySubtitle)
	}
	if bob.PublicIdentifier != "bob-williams" {
		t.Errorf("items[1].PublicIdentifier = %q, want %q", bob.PublicIdentifier, "bob-williams")
	}
}

func TestSearchPeople_EmptyQuery(t *testing.T) {
	c, _ := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
	)
	_, err := NewLinkedIn(c).SearchPeople(context.Background(), "  ", 0, 10)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

// ---------------------------------------------------------------------------
// ListProfilePosts (with mock HTTP server)
// ---------------------------------------------------------------------------

const postListFixture = `{
	"data": {},
	"included": [
		{
			"$type": "com.linkedin.voyager.feed.render.UpdateV2",
			"entityUrn": "urn:li:fs_update:(urn:li:activity:7000000000000000001,MEMBER_SHARE,EMPTY,DEFAULT,false)",
			"updateType": "MEMBER_SHARE",
			"actor": {
				"entityUrn": "urn:li:fs_miniProfile:ACoAAB12345"
			},
			"publishedAt": 1706000000000,
			"commentary": {
				"text": "Excited to share my latest project!"
			}
		},
		{
			"$type": "com.linkedin.voyager.feed.render.UpdateV2",
			"entityUrn": "urn:li:fs_update:(urn:li:activity:7000000000000000002,MEMBER_SHARE,EMPTY,DEFAULT,false)",
			"updateType": "MEMBER_SHARE",
			"actor": {
				"entityUrn": "urn:li:fs_miniProfile:ACoAAB12345"
			},
			"publishedAt": 1705900000000,
			"commentary": {
				"text": "Great conference today!"
			}
		},
		{
			"$type": "com.linkedin.voyager.common.Industry",
			"entityUrn": "urn:li:fs_industry:96"
		}
	]
}`

func TestListProfilePosts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voyager/api/feed/dash/updates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if got := r.URL.Query().Get("profileUrn"); got != "urn:li:member:67890" {
			t.Errorf("profileUrn = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, postListFixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	posts, err := NewLinkedIn(c).ListProfilePosts(context.Background(), "urn:li:member:67890", 0, 10)
	if err != nil {
		t.Fatalf("ListProfilePosts() error: %v", err)
	}

	// Should find 2 Update items from included[], skipping Industry
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}

	if posts[0].Commentary != "Excited to share my latest project!" {
		t.Errorf("posts[0].Commentary = %q", posts[0].Commentary)
	}
	if posts[0].UpdateType != "MEMBER_SHARE" {
		t.Errorf("posts[0].UpdateType = %q", posts[0].UpdateType)
	}
	if posts[0].PublishedAt != 1706000000000 {
		t.Errorf("posts[0].PublishedAt = %d", posts[0].PublishedAt)
	}
	if posts[0].ActorURN != "urn:li:fs_miniProfile:ACoAAB12345" {
		t.Errorf("posts[0].ActorURN = %q", posts[0].ActorURN)
	}

	if posts[1].Commentary != "Great conference today!" {
		t.Errorf("posts[1].Commentary = %q", posts[1].Commentary)
	}
}

func TestListProfilePosts_WithElements(t *testing.T) {
	// Test when posts come in "elements" array directly (non-normalized format)
	fixture := `{
		"elements": [
			{
				"entityUrn": "urn:li:activity:111",
				"updateType": "MEMBER_SHARE",
				"publishedAt": 1706100000000,
				"commentary": {"text": "Direct element post"}
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	posts, err := NewLinkedIn(c).ListProfilePosts(context.Background(), "urn:li:member:1", 0, 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(posts))
	}
	if posts[0].Commentary != "Direct element post" {
		t.Errorf("Commentary = %q", posts[0].Commentary)
	}
}

func TestListProfilePosts_EmptyProfileURN(t *testing.T) {
	c, _ := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
	)
	_, err := NewLinkedIn(c).ListProfilePosts(context.Background(), "", 0, 10)
	if err == nil {
		t.Fatal("expected error for empty profile URN")
	}
}

// ---------------------------------------------------------------------------
// HTTP error handling
// ---------------------------------------------------------------------------

func TestGetMe_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"Not authenticated"}`)
	}))
	defer ts.Close()

	c, _ := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)

	_, err := NewLinkedIn(c).GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error for 403")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", httpErr.StatusCode)
	}
}

func TestGetMe_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c, _ := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)

	_, err := NewLinkedIn(c).GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error for 429")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", httpErr.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Client options
// ---------------------------------------------------------------------------

func TestNewClient_MissingCookies(t *testing.T) {
	c, err := NewClient(auth.Cookies{})
	if err != nil {
		t.Fatalf("NewClient should not fail on empty cookies at construction: %v", err)
	}
	// But Do should fail
	err = c.Do(context.Background(), "GET", "/me", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing cookies in Do()")
	}
}

func TestNewClient_WithDebug(t *testing.T) {
	var buf []byte
	w := &writerFunc{fn: func(p []byte) (int, error) {
		buf = append(buf, p...)
		return len(p), nil
	}}

	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		_, _ = io.WriteString(rw, `{}`)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "x", JSessionID: "ajax:y"},
		WithBaseURL(ts.URL+"/voyager/api"),
		WithDebug(w),
	)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = c.Do(context.Background(), "GET", "/test", nil, nil, &out)

	if len(buf) == 0 {
		t.Error("expected debug output, got none")
	}
}

type writerFunc struct {
	fn func([]byte) (int, error)
}

func (w *writerFunc) Write(p []byte) (int, error) { return w.fn(p) }

// ---------------------------------------------------------------------------
// JSON fixture sanity: make sure our fixtures parse cleanly
// ---------------------------------------------------------------------------

func TestFixtures_ValidJSON(t *testing.T) {
	fixtures := map[string]string{
		"getMeFixture":      getMeFixture,
		"getProfileFixture": getProfileFixture,
		"searchFixture":     searchFixture,
		"postListFixture":   postListFixture,
	}
	for name, f := range fixtures {
		var v any
		if err := json.Unmarshal([]byte(f), &v); err != nil {
			t.Errorf("fixture %s is not valid JSON: %v", name, err)
		}
	}
}
