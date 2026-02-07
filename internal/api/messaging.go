package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	// DefaultConversationsQueryID is the GraphQL query ID for listing conversations.
	DefaultConversationsQueryID = "messengerConversations.9501074288a12f3ae9e3c7ea243bccbf"

	// DefaultMessagesQueryID is the GraphQL query ID for fetching messages in a conversation.
	DefaultMessagesQueryID = "messengerMessages.5846eeb71c981f11e0134cb6626cc314"

	// messagingGraphQLPath is the path (relative to BaseURL) for messaging GraphQL.
	messagingGraphQLPath = "voyagerMessagingGraphQL/graphql"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Conversation represents a LinkedIn messaging conversation.
type Conversation struct {
	EntityURN    string
	Participants []Participant
	LastMessage  *Message
}

// Participant represents a participant in a conversation.
type Participant struct {
	EntityURN  string // messaging participant URN
	FirstName  string
	LastName   string
	ProfileURN string // urn:li:fsd_profile:… (hostIdentityUrn)
}

// FullName returns "First Last", trimmed.
func (p Participant) FullName() string {
	return strings.TrimSpace(p.FirstName + " " + p.LastName)
}

// Message represents a single message in a conversation.
type Message struct {
	EntityURN   string
	BodyText    string
	SenderURN   string // messaging participant URN of sender
	SenderName  string // resolved "First Last"
	DeliveredAt int64  // millisecond epoch
}

// ---------------------------------------------------------------------------
// URL encoding helpers
// ---------------------------------------------------------------------------

// encodeURNValue percent-encodes the special characters inside a URN value
// for use within LinkedIn's tuple-syntax variables. The structural tuple
// characters (outer parens, colons, commas) remain literal in the query
// string, but URN values embedded inside must be encoded.
func encodeURNValue(urn string) string {
	r := strings.NewReplacer(
		":", "%3A",
		"(", "%28",
		")", "%29",
		",", "%2C",
	)
	return r.Replace(urn)
}

// ---------------------------------------------------------------------------
// Query ID helpers
// ---------------------------------------------------------------------------

func (li *LinkedIn) conversationsQueryID() string {
	if li.ConversationsQueryID != "" {
		return li.ConversationsQueryID
	}
	return DefaultConversationsQueryID
}

func (li *LinkedIn) messagesQueryID() string {
	if li.MessagesQueryID != "" {
		return li.MessagesQueryID
	}
	return DefaultMessagesQueryID
}

// ---------------------------------------------------------------------------
// API methods
// ---------------------------------------------------------------------------

// ListConversations fetches the user's inbox conversations.
// profileURN must be the user's own urn:li:fsd_profile:… URN.
func (li *LinkedIn) ListConversations(ctx context.Context, profileURN string, count int) ([]Conversation, error) {
	if strings.TrimSpace(profileURN) == "" {
		return nil, fmt.Errorf("empty profile URN")
	}
	if count <= 0 {
		count = 20
	}

	encodedURN := encodeURNValue(profileURN)
	variables := fmt.Sprintf(
		"(query:(predicateUnions:List((conversationCategoryPredicate:(category:INBOX)))),count:%d,mailboxUrn:%s)",
		count, encodedURN,
	)

	rawQuery := fmt.Sprintf("variables=%s&queryId=%s", variables, li.conversationsQueryID())

	var raw map[string]any
	if err := li.c.DoRaw(ctx, "GET", messagingGraphQLPath, rawQuery, nil, &raw); err != nil {
		return nil, err
	}

	return ParseConversations(raw), nil
}

// GetMessages fetches messages in a conversation.
func (li *LinkedIn) GetMessages(ctx context.Context, conversationURN string, count int) ([]Message, error) {
	if strings.TrimSpace(conversationURN) == "" {
		return nil, fmt.Errorf("empty conversation URN")
	}
	_ = count // the default endpoint returns recent messages; count is handled server-side

	encodedURN := encodeURNValue(conversationURN)
	variables := fmt.Sprintf("(conversationUrn:%s)", encodedURN)

	rawQuery := fmt.Sprintf("variables=%s&queryId=%s", variables, li.messagesQueryID())

	var raw map[string]any
	if err := li.c.DoRaw(ctx, "GET", messagingGraphQLPath, rawQuery, nil, &raw); err != nil {
		return nil, err
	}

	return ParseMessages(raw), nil
}

// SendMessage sends a text message to an existing conversation.
// This is experimental — the endpoint is inferred from LinkedIn's dash API patterns.
func (li *LinkedIn) SendMessage(ctx context.Context, mailboxURN, conversationURN, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("empty message text")
	}

	payload := map[string]any{
		"body": map[string]any{
			"text":       text,
			"attributes": []any{},
		},
		"conversationUrn": conversationURN,
		"mailboxUrn":      mailboxURN,
	}

	rawQuery := "action=createMessage"
	return li.c.DoRaw(ctx, "POST", "voyagerMessagingDashMessengerMessages", rawQuery, payload, nil)
}

// CreateConversationWithMessage starts a new conversation with a message.
// recipientURNs are urn:li:fsd_profile:… URNs.
// This is experimental — the endpoint is inferred from LinkedIn's dash API patterns.
func (li *LinkedIn) CreateConversationWithMessage(ctx context.Context, mailboxURN string, recipientURNs []string, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("empty message text")
	}
	if len(recipientURNs) == 0 {
		return fmt.Errorf("no recipients")
	}

	recipients := make([]any, len(recipientURNs))
	for i, r := range recipientURNs {
		recipients[i] = r
	}

	payload := map[string]any{
		"message": map[string]any{
			"body": map[string]any{
				"text":       text,
				"attributes": []any{},
			},
		},
		"recipients":  recipients,
		"mailboxUrn":  mailboxURN,
		"subtype":     "MEMBER_TO_MEMBER",
	}

	rawQuery := "action=create"
	return li.c.DoRaw(ctx, "POST", "voyagerMessagingDashMessengerConversations", rawQuery, payload, nil)
}

// ---------------------------------------------------------------------------
// Response parsing (exported for testing)
// ---------------------------------------------------------------------------

// ParseConversations extracts conversations from a LinkedIn messaging GraphQL response.
func ParseConversations(raw map[string]any) []Conversation {
	included, _ := raw["included"].([]any)
	if len(included) == 0 {
		return nil
	}

	// Phase 1: index participants by entityURN.
	participantsByURN := make(map[string]Participant)
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		if t != "com.linkedin.messenger.MessagingParticipant" {
			continue
		}
		p := parseParticipant(m)
		if p.EntityURN != "" {
			participantsByURN[p.EntityURN] = p
		}
	}

	// Phase 2: index messages by entityURN.
	messagesByURN := make(map[string]Message)
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		if t != "com.linkedin.messenger.Message" {
			continue
		}
		msg := parseMessage(m, participantsByURN)
		if msg.EntityURN != "" {
			messagesByURN[msg.EntityURN] = msg
		}
	}

	// Phase 3: build conversations.
	var convos []Conversation
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		if t != "com.linkedin.messenger.Conversation" {
			continue
		}

		entityURN := getString(m, "entityUrn")
		c := Conversation{EntityURN: entityURN}

		// Resolve participants (try both *-prefixed and non-prefixed keys).
		for _, key := range []string{"*conversationParticipants", "conversationParticipants"} {
			if refs, ok := m[key].([]any); ok {
				for _, ref := range refs {
					if s, ok := ref.(string); ok {
						if p, found := participantsByURN[s]; found {
							c.Participants = append(c.Participants, p)
						}
					}
				}
				if len(c.Participants) > 0 {
					break
				}
			}
		}

		// Resolve last message.
		for _, key := range []string{"*lastMessage", "lastMessage"} {
			if ref, ok := m[key].(string); ok && ref != "" {
				if msg, found := messagesByURN[ref]; found {
					c.LastMessage = &msg
					break
				}
			}
		}

		convos = append(convos, c)
	}

	// Sort by last message timestamp descending (newest first).
	sort.Slice(convos, func(i, j int) bool {
		ti, tj := int64(0), int64(0)
		if convos[i].LastMessage != nil {
			ti = convos[i].LastMessage.DeliveredAt
		}
		if convos[j].LastMessage != nil {
			tj = convos[j].LastMessage.DeliveredAt
		}
		return ti > tj
	})

	return convos
}

// ParseMessages extracts messages from a LinkedIn messaging GraphQL response.
func ParseMessages(raw map[string]any) []Message {
	included, _ := raw["included"].([]any)
	if len(included) == 0 {
		return nil
	}

	// Index participants for sender name resolution.
	participantsByURN := make(map[string]Participant)
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		if t != "com.linkedin.messenger.MessagingParticipant" {
			continue
		}
		p := parseParticipant(m)
		if p.EntityURN != "" {
			participantsByURN[p.EntityURN] = p
		}
	}

	var msgs []Message
	for _, item := range included {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["$type"].(string)
		if t != "com.linkedin.messenger.Message" {
			continue
		}
		msgs = append(msgs, parseMessage(m, participantsByURN))
	}

	// Sort by deliveredAt ascending (chronological reading order).
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].DeliveredAt < msgs[j].DeliveredAt
	})

	return msgs
}

// ---------------------------------------------------------------------------
// Internal parsing helpers
// ---------------------------------------------------------------------------

func parseParticipant(m map[string]any) Participant {
	p := Participant{
		EntityURN: getString(m, "entityUrn"),
	}

	// hostIdentityUrn → fsd_profile URN
	p.ProfileURN = getString(m, "hostIdentityUrn")

	// Name: participantType → member → firstName/lastName → text
	if pt, ok := m["participantType"].(map[string]any); ok {
		if member, ok := pt["member"].(map[string]any); ok {
			if fn, ok := member["firstName"].(map[string]any); ok {
				p.FirstName, _ = fn["text"].(string)
			}
			if ln, ok := member["lastName"].(map[string]any); ok {
				p.LastName, _ = ln["text"].(string)
			}
		}
	}

	return p
}

func parseMessage(m map[string]any, participants map[string]Participant) Message {
	msg := Message{
		EntityURN:   getString(m, "entityUrn"),
		DeliveredAt: getInt64(m, "deliveredAt"),
	}

	if body, ok := m["body"].(map[string]any); ok {
		msg.BodyText, _ = body["text"].(string)
	}

	// Sender reference: try *sender then sender.
	msg.SenderURN = getString(m, "*sender")
	if msg.SenderURN == "" {
		msg.SenderURN, _ = m["sender"].(string)
	}

	// Resolve sender name.
	if p, ok := participants[msg.SenderURN]; ok {
		msg.SenderName = p.FullName()
	}

	return msg
}

// FindConversationByProfileURN finds a conversation that includes a
// participant whose hostIdentityUrn matches the given profile URN.
func FindConversationByProfileURN(convos []Conversation, profileURN string) *Conversation {
	for i, c := range convos {
		for _, p := range c.Participants {
			if p.ProfileURN == profileURN {
				return &convos[i]
			}
		}
	}
	return nil
}
