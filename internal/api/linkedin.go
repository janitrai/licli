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

	miniEntityURN := getString(raw, "miniProfile", "entityUrn")
	if miniEntityURN == "" {
		miniEntityURN = getString(raw, "data", "miniProfile", "entityUrn")
	}

	publicID := getString(raw, "miniProfile", "publicIdentifier")
	if publicID == "" {
		publicID = getString(raw, "data", "miniProfile", "publicIdentifier")
	}

	first := getString(raw, "miniProfile", "firstName")
	last := getString(raw, "miniProfile", "lastName")
	if first == "" && last == "" {
		first = getString(raw, "data", "miniProfile", "firstName")
		last = getString(raw, "data", "miniProfile", "lastName")
	}

	occupation := getString(raw, "miniProfile", "occupation")
	if occupation == "" {
		occupation = getString(raw, "data", "miniProfile", "occupation")
	}

	memberID := urnID(miniEntityURN)
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
	path := fmt.Sprintf("/identity/profiles/%s/profileView", url.PathEscape(id))
	if err := li.c.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return Profile{}, err
	}

	profilePublicID := getString(raw, "profile", "miniProfile", "publicIdentifier")
	if profilePublicID == "" {
		profilePublicID = getString(raw, "profile", "publicIdentifier")
	}

	first := getString(raw, "profile", "firstName")
	last := getString(raw, "profile", "lastName")
	headline := getString(raw, "profile", "headline")
	summary := getString(raw, "profile", "summary")
	location := getString(raw, "profile", "locationName")

	miniEntityURN := getString(raw, "profile", "miniProfile", "entityUrn")
	if miniEntityURN == "" {
		miniEntityURN = getString(raw, "profile", "miniProfile", "entityURN")
	}
	memberID := urnID(miniEntityURN)
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
		MiniProfileEntityURN: miniEntityURN,
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

func (li *LinkedIn) SearchPeople(ctx context.Context, keywords string, start, count int) ([]SearchItem, error) {
	filters := "List(resultType->PEOPLE)"
	return li.searchBlended(ctx, keywords, filters, start, count)
}

func (li *LinkedIn) SearchJobs(ctx context.Context, keywords string, start, count int) ([]SearchItem, error) {
	filters := "List(resultType->JOBS)"
	return li.searchBlended(ctx, keywords, filters, start, count)
}

func (li *LinkedIn) searchBlended(ctx context.Context, keywords string, filters string, start, count int) ([]SearchItem, error) {
	if strings.TrimSpace(keywords) == "" {
		return nil, fmt.Errorf("empty query")
	}
	if count <= 0 {
		count = 10
	}
	if start < 0 {
		start = 0
	}

	q := url.Values{}
	q.Set("count", fmt.Sprintf("%d", count))
	q.Set("filters", filters)
	q.Set("origin", "GLOBAL_SEARCH_HEADER")
	q.Set("q", "all")
	q.Set("start", fmt.Sprintf("%d", start))
	q.Set("queryContext", "List(spellCorrectionEnabled->true,relatedSearchesEnabled->true,kcardTypes->PROFILE|COMPANY)")
	q.Set("keywords", keywords)

	var raw map[string]any
	if err := li.c.Do(ctx, "GET", "/search/blended", q, nil, &raw); err != nil {
		return nil, err
	}

	// Expected structure (observed in other implementations):
	// data.elements[*].elements[*] -> individual results
	data, ok := raw["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected search response")
	}
	outer, _ := data["elements"].([]any)

	var items []SearchItem
	for _, o := range outer {
		om, ok := o.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := om["elements"].([]any)
		for _, it := range inner {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}

			publicID := getString(m, "publicIdentifier")
			title := getString(m, "title", "text")
			primary := getString(m, "primarySubtitle", "text")
			secondary := getString(m, "secondarySubtitle", "text")
			target := getString(m, "targetUrn")
			if target == "" {
				target = getString(m, "trackingUrn")
			}

			items = append(items, SearchItem{
				PublicIdentifier:  publicID,
				Title:             title,
				PrimarySubtitle:   primary,
				SecondarySubtitle: secondary,
				TargetURN:         target,
			})
		}
	}

	return items, nil
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
