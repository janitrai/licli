package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/horsefit/li/internal/auth"
)

type LinkedIn struct {
	c *Client
}

func NewLinkedIn(c *Client) *LinkedIn {
	return &LinkedIn{c: c}
}

type Me struct {
	PublicIdentifier string
	FirstName        string
	LastName         string
	Occupation       string

	MiniProfileEntityURN string
	MemberID             string
	MemberURN            string
}

func (li *LinkedIn) GetMe(ctx context.Context) (Me, error) {
	var raw map[string]any
	if err := li.c.Do(ctx, "GET", "/me", nil, nil, &raw); err != nil {
		return Me{}, err
	}

	// The /me response uses LinkedIn's normalized format:
	//   data.*miniProfile → URN reference
	//   included[] → array of resolved entities (miniProfile lives here)
	mini := findMiniProfile(raw)

	publicID := getStringFrom(mini, "publicIdentifier")
	first := getStringFrom(mini, "firstName")
	last := getStringFrom(mini, "lastName")
	occupation := getStringFrom(mini, "occupation")
	miniEntityURN := getStringFrom(mini, "entityUrn")
	if miniEntityURN == "" {
		miniEntityURN = getStringFrom(mini, "dashEntityUrn")
	}

	memberID := urnID(miniEntityURN)
	if memberID == "" {
		// Try objectUrn: "urn:li:member:123"
		memberID = urnID(getStringFrom(mini, "objectUrn"))
	}
	memberURN := ""
	if memberID != "" {
		memberURN = "urn:li:member:" + memberID
	}

	return Me{
		PublicIdentifier:     publicID,
		FirstName:            first,
		LastName:             last,
		Occupation:           occupation,
		MiniProfileEntityURN: miniEntityURN,
		MemberID:             memberID,
		MemberURN:            memberURN,
	}, nil
}

// findMiniProfile extracts the miniProfile object from LinkedIn's normalized response.
// It checks included[] first (normalized format), then falls back to nested paths.
func findMiniProfile(raw map[string]any) map[string]any {
	// Check included[] array for a miniProfile entity
	if included, ok := raw["included"].([]any); ok {
		for _, item := range included {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			// miniProfiles have entityUrn containing "fs_miniProfile" or "fsd_profile"
			urn, _ := m["entityUrn"].(string)
			dashUrn, _ := m["dashEntityUrn"].(string)
			if strings.Contains(urn, "miniProfile") || strings.Contains(dashUrn, "fsd_profile") {
				return m
			}
			// Also check $type
			if t, _ := m["$type"].(string); strings.Contains(t, "MiniProfile") || strings.Contains(t, "miniProfile") {
				return m
			}
		}
	}
	// Fallback: nested miniProfile under data or top-level
	if data, ok := raw["data"].(map[string]any); ok {
		if mp, ok := data["miniProfile"].(map[string]any); ok {
			return mp
		}
	}
	if mp, ok := raw["miniProfile"].(map[string]any); ok {
		return mp
	}
	return nil
}

// findProfileInIncluded finds the main profile entity from included[] in the dash API response.
func findProfileInIncluded(raw map[string]any) map[string]any {
	included, _ := raw["included"].([]any)
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		urn, _ := m["entityUrn"].(string)
		if strings.Contains(t, "Profile") && strings.Contains(urn, "fsd_profile") {
			return m
		}
	}
	// Fallback: any item with firstName
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := m["firstName"]; ok {
			return m
		}
	}
	return nil
}

func getStringFrom(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

type Profile struct {
	PublicIdentifier string
	FirstName        string
	LastName         string
	Headline         string
	Summary          string
	LocationName     string

	MiniProfileEntityURN string
	MemberID             string
	MemberURN            string
}

func (li *LinkedIn) GetProfile(ctx context.Context, publicIdentifierOrURN string) (Profile, error) {
	id := strings.TrimSpace(publicIdentifierOrURN)
	if id == "" {
		return Profile{}, fmt.Errorf("empty profile identifier")
	}

	var raw map[string]any
	// Use the dash API (the old /identity/profiles/{id}/profileView is deprecated/410)
	query := url.Values{"q": {"memberIdentity"}, "memberIdentity": {id}}
	if err := li.c.Do(ctx, "GET", "/identity/dash/profiles", query, nil, &raw); err != nil {
		return Profile{}, err
	}

	// The dash API returns a normalized response with profile data in included[]
	prof := findProfileInIncluded(raw)

	profilePublicID := getStringFrom(prof, "publicIdentifier")
	first := getStringFrom(prof, "firstName")
	last := getStringFrom(prof, "lastName")
	headline := getStringFrom(prof, "headline")
	summary := getStringFrom(prof, "summary")
	location := getStringFrom(prof, "geoLocationName")
	if location == "" {
		location = getStringFrom(prof, "locationName")
	}

	entityURN := getStringFrom(prof, "entityUrn")
	if entityURN == "" {
		entityURN = getStringFrom(prof, "dashEntityUrn")
	}
	memberID := urnID(entityURN)
	if memberID == "" {
		memberID = urnID(getStringFrom(prof, "objectUrn"))
	}
	memberURN := ""
	if memberID != "" {
		memberURN = "urn:li:member:" + memberID
	}

	return Profile{
		PublicIdentifier:     profilePublicID,
		FirstName:            first,
		LastName:             last,
		Headline:             headline,
		Summary:              summary,
		LocationName:         location,
		MiniProfileEntityURN: entityURN,
		MemberID:             memberID,
		MemberURN:            memberURN,
	}, nil
}

type CreatePostResult struct {
	EntityURN string
}

func (li *LinkedIn) CreatePost(ctx context.Context, ownerURN string, text string) (CreatePostResult, error) {
	if strings.TrimSpace(text) == "" {
		return CreatePostResult{}, fmt.Errorf("post text is empty")
	}

	trackingID, _ := auth.RandomTrackingID()

	payload := map[string]any{
		"commentary": map[string]any{
			"text": text,
		},
		"visibility": "PUBLIC",
		"distribution": map[string]any{
			"feedDistribution":                "MAIN_FEED",
			"targetEntities":                  []any{},
			"thirdPartyDistributionChannels":  []any{},
			"thirdPartyDistributionTargeting": []any{},
		},
		"lifecycleState":            "PUBLISHED",
		"isReshareDisabledByAuthor": false,
		"shareMediaCategory":        "NONE",
	}
	if trackingID != "" {
		payload["trackingId"] = trackingID
	}
	if ownerURN != "" {
		// Not always required, but helps ensure the post is created for the authenticated member.
		payload["owner"] = ownerURN
	}

	var raw map[string]any
	if err := li.c.Do(ctx, "POST", "/contentcreation/normShares", nil, payload, &raw); err != nil {
		return CreatePostResult{}, err
	}

	entityURN := getString(raw, "entityUrn")
	if entityURN == "" {
		entityURN = getString(raw, "data", "entityUrn")
	}
	if entityURN == "" {
		entityURN = findFirstString(raw, "entityUrn")
	}
	return CreatePostResult{EntityURN: entityURN}, nil
}

type FeedUpdate struct {
	EntityURN   string
	Commentary  string
	UpdateType  string
	ActorURN    string
	PublishedAt int64
}

func (li *LinkedIn) ListProfilePosts(ctx context.Context, profileID string, start, count int) ([]FeedUpdate, error) {
	if strings.TrimSpace(profileID) == "" {
		return nil, fmt.Errorf("empty profile id")
	}
	if count <= 0 {
		count = 10
	}
	if start < 0 {
		start = 0
	}

	q := url.Values{}
	q.Set("q", "memberShareFeed")
	q.Set("moduleKey", "member-share")
	q.Set("count", fmt.Sprintf("%d", count))
	q.Set("start", fmt.Sprintf("%d", start))
	q.Set("profileId", profileID)

	var raw map[string]any
	if err := li.c.Do(ctx, "GET", "/feed/updates", q, nil, &raw); err != nil {
		return nil, err
	}

	elements, _ := raw["elements"].([]any)
	out := make([]FeedUpdate, 0, len(elements))
	for _, el := range elements {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		entityURN := getString(m, "entityUrn")
		updateType := getString(m, "updateType")
		actorURN := getString(m, "actor", "entityUrn")
		publishedAt := getInt64(m, "publishedAt")
		commentary := findCommentaryText(m)

		out = append(out, FeedUpdate{
			EntityURN:   entityURN,
			UpdateType:  updateType,
			ActorURN:    actorURN,
			PublishedAt: publishedAt,
			Commentary:  commentary,
		})
	}

	return out, nil
}

type SearchItem struct {
	PublicIdentifier  string
	Title             string
	PrimarySubtitle   string
	SecondarySubtitle string
	TargetURN         string
}

// SearchQueryID is the GraphQL query ID for search clusters.
// LinkedIn rotates these periodically; update if search returns 500.
const SearchQueryID = "voyagerSearchDashClusters.ef3d0937fb65bd7812e32e5a85028e79"

func (li *LinkedIn) SearchPeople(ctx context.Context, keywords string, start, count int) ([]SearchItem, error) {
	return li.searchGraphQL(ctx, keywords, "PEOPLE", start, count)
}

func (li *LinkedIn) SearchJobs(ctx context.Context, keywords string, start, count int) ([]SearchItem, error) {
	return li.searchGraphQL(ctx, keywords, "JOBS", start, count)
}

func (li *LinkedIn) searchGraphQL(ctx context.Context, keywords string, resultType string, start, count int) ([]SearchItem, error) {
	if strings.TrimSpace(keywords) == "" {
		return nil, fmt.Errorf("empty query")
	}
	if count <= 0 {
		count = 10
	}
	if start < 0 {
		start = 0
	}

	// Build the LinkedIn-style variables tuple.
	// LinkedIn uses a custom tuple syntax that must NOT be percent-encoded for parens/commas/colons.
	// Only the keywords value needs %20 encoding for spaces.
	escapedKW := strings.ReplaceAll(url.PathEscape(keywords), "+", "%20")
	variables := fmt.Sprintf(
		"(start:%d,origin:OTHER,query:(keywords:%s,flagshipSearchIntent:SEARCH_SRP,queryParameters:List((key:resultType,value:List(%s))),includeFiltersInResponse:false))",
		start, escapedKW, resultType,
	)

	// Build the raw query string manually to avoid double-encoding the tuple syntax
	rawQuery := fmt.Sprintf("includeWebMetadata=true&variables=%s&queryId=%s",
		variables, SearchQueryID)

	var raw map[string]any
	if err := li.c.DoRaw(ctx, "GET", "/graphql", rawQuery, nil, &raw); err != nil {
		return nil, err
	}

	// Results are in included[] as EntityResultViewModel objects
	included, _ := raw["included"].([]any)
	var items []SearchItem
	for _, el := range included {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		if !strings.Contains(t, "EntityResultViewModel") {
			continue
		}

		title := getNestedText(m, "title")
		primary := getNestedText(m, "primarySubtitle")
		secondary := getNestedText(m, "secondarySubtitle")
		targetURN, _ := m["entityUrn"].(string)

		// Try to extract publicIdentifier from the navigation URL
		publicID := ""
		if navURL := getString(m, "navigationUrl"); navURL != "" {
			publicID = auth.NormalizePublicIdentifier(navURL)
		}

		items = append(items, SearchItem{
			PublicIdentifier:  publicID,
			Title:             title,
			PrimarySubtitle:   primary,
			SecondarySubtitle: secondary,
			TargetURN:         targetURN,
		})
	}

	return items, nil
}

// getNestedText extracts .text from a field that may be a string or {text: "..."} object.
func getNestedText(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		s, _ := t["text"].(string)
		return s
	default:
		return ""
	}
}

func (li *LinkedIn) Follow(ctx context.Context, memberURN string) error {
	memberURN = strings.TrimSpace(memberURN)
	if memberURN == "" {
		return fmt.Errorf("empty member urn")
	}
	if !strings.HasPrefix(memberURN, "urn:li:member:") {
		return fmt.Errorf("unexpected member urn: %q", memberURN)
	}

	followingInfoURN := "urn:li:fs_followingInfo:" + memberURN
	payload := map[string]any{"urn": followingInfoURN}

	q := url.Values{}
	q.Set("action", "followByEntityUrn")
	return li.c.Do(ctx, "POST", "/feed/follows", q, payload, nil)
}

func (li *LinkedIn) Connect(ctx context.Context, profileID string, note string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("empty profile id")
	}

	trackingID, _ := auth.RandomTrackingID()
	payload := map[string]any{
		"emberEntityName": "growth/invitation/norm-invitation",
		"invitee": map[string]any{
			"com.linkedin.voyager.growth.invitation.InviteeProfile": map[string]any{
				"profileId": profileID,
			},
		},
	}
	if trackingID != "" {
		payload["trackingId"] = trackingID
	}
	if strings.TrimSpace(note) != "" {
		payload["customMessage"] = note
	}

	return li.c.Do(ctx, "POST", "/growth/normInvitations", nil, payload, nil)
}

func urnID(urn string) string {
	urn = strings.TrimSpace(urn)
	if urn == "" {
		return ""
	}
	if i := strings.LastIndexByte(urn, ':'); i >= 0 && i+1 < len(urn) {
		return urn[i+1:]
	}
	return ""
}

func getString(m map[string]any, path ...string) string {
	var cur any = m
	for _, p := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = next[p]
		if !ok {
			return ""
		}
	}
	s, _ := cur.(string)
	return s
}

func getInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

func findCommentaryText(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if c, ok := t["commentary"]; ok {
			if txt := findTextField(c); txt != "" {
				return txt
			}
		}
		if c, ok := t["shareCommentary"]; ok {
			if txt := findTextField(c); txt != "" {
				return txt
			}
		}
		for _, vv := range t {
			if txt := findCommentaryText(vv); txt != "" {
				return txt
			}
		}
	case []any:
		for _, vv := range t {
			if txt := findCommentaryText(vv); txt != "" {
				return txt
			}
		}
	}
	return ""
}

func findTextField(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := t["text"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
		for _, vv := range t {
			if s := findTextField(vv); s != "" {
				return s
			}
		}
	case []any:
		for _, vv := range t {
			if s := findTextField(vv); s != "" {
				return s
			}
		}
	}
	return ""
}

func findFirstString(v any, key string) string {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := t[key].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
		for _, vv := range t {
			if s := findFirstString(vv, key); s != "" {
				return s
			}
		}
	case []any:
		for _, vv := range t {
			if s := findFirstString(vv, key); s != "" {
				return s
			}
		}
	}
	return ""
}
