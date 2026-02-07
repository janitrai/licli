package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/horsefit/li/internal/auth"
)

// ---------------------------------------------------------------------------
// encodeURNValue
// ---------------------------------------------------------------------------

func TestEncodeURNValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"simple fsd_profile URN",
			"urn:li:fsd_profile:ACoAAGQ0PMsBs6zsDe1dkmea",
			"urn%3Ali%3Afsd_profile%3AACoAAGQ0PMsBs6zsDe1dkmea",
		},
		{
			"conversation URN with nested parens and comma",
			"urn:li:msg_conversation:(urn:li:fsd_profile:AAA,2-MjkzM)",
			"urn%3Ali%3Amsg_conversation%3A%28urn%3Ali%3Afsd_profile%3AAAA%2C2-MjkzM%29",
		},
		{
			"empty string",
			"",
			"",
		},
		{
			"no special chars",
			"plaintext",
			"plaintext",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeURNValue(tt.in)
			if got != tt.want {
				t.Errorf("encodeURNValue(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParseConversations
// ---------------------------------------------------------------------------

const conversationsFixture = `{
	"included": [
		{
			"$type": "com.linkedin.messenger.MessagingParticipant",
			"entityUrn": "urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:BBB)",
			"hostIdentityUrn": "urn:li:fsd_profile:BBB",
			"participantType": {
				"member": {
					"firstName": {"text": "Jane"},
					"lastName": {"text": "Doe"}
				}
			}
		},
		{
			"$type": "com.linkedin.messenger.MessagingParticipant",
			"entityUrn": "urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:AAA)",
			"hostIdentityUrn": "urn:li:fsd_profile:AAA",
			"participantType": {
				"member": {
					"firstName": {"text": "Me"},
					"lastName": {"text": "Myself"}
				}
			}
		},
		{
			"$type": "com.linkedin.messenger.MessagingParticipant",
			"entityUrn": "urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:CCC)",
			"hostIdentityUrn": "urn:li:fsd_profile:CCC",
			"participantType": {
				"member": {
					"firstName": {"text": "Bob"},
					"lastName": {"text": "Smith"}
				}
			}
		},
		{
			"$type": "com.linkedin.messenger.Message",
			"entityUrn": "urn:li:msg_message:(urn:li:fsd_profile:AAA,msg001)",
			"body": {"text": "Hey, how are you?"},
			"*sender": "urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:BBB)",
			"deliveredAt": 1707321600000
		},
		{
			"$type": "com.linkedin.messenger.Message",
			"entityUrn": "urn:li:msg_message:(urn:li:fsd_profile:AAA,msg002)",
			"body": {"text": "Let's catch up soon!"},
			"*sender": "urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:CCC)",
			"deliveredAt": 1707300000000
		},
		{
			"$type": "com.linkedin.messenger.Conversation",
			"entityUrn": "urn:li:msg_conversation:(urn:li:fsd_profile:AAA,thread001)",
			"*conversationParticipants": [
				"urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:AAA)",
				"urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:BBB)"
			],
			"*lastMessage": "urn:li:msg_message:(urn:li:fsd_profile:AAA,msg001)"
		},
		{
			"$type": "com.linkedin.messenger.Conversation",
			"entityUrn": "urn:li:msg_conversation:(urn:li:fsd_profile:AAA,thread002)",
			"*conversationParticipants": [
				"urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:AAA)",
				"urn:li:msg_participant:(urn:li:fsd_profile:AAA,urn:li:fsd_profile:CCC)"
			],
			"*lastMessage": "urn:li:msg_message:(urn:li:fsd_profile:AAA,msg002)"
		}
	]
}`

func TestParseConversations(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(conversationsFixture), &raw); err != nil {
		t.Fatalf("bad fixture JSON: %v", err)
	}

	convos := ParseConversations(raw)
	if len(convos) != 2 {
		t.Fatalf("got %d conversations, want 2", len(convos))
	}

	// Sorted newest first: thread001 (1707321600000) before thread002 (1707300000000).
	c1 := convos[0]
	if c1.EntityURN != "urn:li:msg_conversation:(urn:li:fsd_profile:AAA,thread001)" {
		t.Errorf("convos[0].EntityURN = %q", c1.EntityURN)
	}
	if len(c1.Participants) != 2 {
		t.Fatalf("convos[0] has %d participants, want 2", len(c1.Participants))
	}
	// Check participant name resolution
	foundJane := false
	for _, p := range c1.Participants {
		if p.FirstName == "Jane" && p.LastName == "Doe" {
			foundJane = true
			if p.ProfileURN != "urn:li:fsd_profile:BBB" {
				t.Errorf("Jane's ProfileURN = %q", p.ProfileURN)
			}
		}
	}
	if !foundJane {
		t.Error("expected to find participant Jane Doe in convos[0]")
	}

	if c1.LastMessage == nil {
		t.Fatal("convos[0].LastMessage is nil")
	}
	if c1.LastMessage.BodyText != "Hey, how are you?" {
		t.Errorf("convos[0].LastMessage.BodyText = %q", c1.LastMessage.BodyText)
	}
	if c1.LastMessage.SenderName != "Jane Doe" {
		t.Errorf("convos[0].LastMessage.SenderName = %q", c1.LastMessage.SenderName)
	}
	if c1.LastMessage.DeliveredAt != 1707321600000 {
		t.Errorf("convos[0].LastMessage.DeliveredAt = %d", c1.LastMessage.DeliveredAt)
	}

	c2 := convos[1]
	if c2.LastMessage == nil {
		t.Fatal("convos[1].LastMessage is nil")
	}
	if c2.LastMessage.BodyText != "Let's catch up soon!" {
		t.Errorf("convos[1].LastMessage.BodyText = %q", c2.LastMessage.BodyText)
	}
	if c2.LastMessage.SenderName != "Bob Smith" {
		t.Errorf("convos[1].LastMessage.SenderName = %q", c2.LastMessage.SenderName)
	}
}

func TestParseConversations_EmptyIncluded(t *testing.T) {
	raw := map[string]any{"included": []any{}}
	convos := ParseConversations(raw)
	if convos != nil {
		t.Errorf("expected nil, got %d conversations", len(convos))
	}
}

func TestParseConversations_NoIncluded(t *testing.T) {
	raw := map[string]any{"data": map[string]any{}}
	convos := ParseConversations(raw)
	if convos != nil {
		t.Errorf("expected nil, got %v", convos)
	}
}

func TestParseConversations_UnprefixedKeys(t *testing.T) {
	// Test with non-*-prefixed field names (conversationParticipants, lastMessage).
	fixture := `{
		"included": [
			{
				"$type": "com.linkedin.messenger.MessagingParticipant",
				"entityUrn": "urn:li:msg_participant:p1",
				"hostIdentityUrn": "urn:li:fsd_profile:P1",
				"participantType": {"member": {"firstName": {"text": "Alice"}, "lastName": {"text": "W"}}}
			},
			{
				"$type": "com.linkedin.messenger.Message",
				"entityUrn": "urn:li:msg_message:m1",
				"body": {"text": "Using unprefixed keys"},
				"sender": "urn:li:msg_participant:p1",
				"deliveredAt": 1700000000000
			},
			{
				"$type": "com.linkedin.messenger.Conversation",
				"entityUrn": "urn:li:msg_conversation:c1",
				"conversationParticipants": ["urn:li:msg_participant:p1"],
				"lastMessage": "urn:li:msg_message:m1"
			}
		]
	}`
	var raw map[string]any
	if err := json.Unmarshal([]byte(fixture), &raw); err != nil {
		t.Fatal(err)
	}
	convos := ParseConversations(raw)
	if len(convos) != 1 {
		t.Fatalf("got %d conversations, want 1", len(convos))
	}
	if len(convos[0].Participants) != 1 {
		t.Errorf("participants = %d, want 1", len(convos[0].Participants))
	}
	if convos[0].LastMessage == nil {
		t.Fatal("LastMessage is nil")
	}
	if convos[0].LastMessage.BodyText != "Using unprefixed keys" {
		t.Errorf("BodyText = %q", convos[0].LastMessage.BodyText)
	}
	// sender was stored without * prefix
	if convos[0].LastMessage.SenderName != "Alice W" {
		t.Errorf("SenderName = %q, want %q", convos[0].LastMessage.SenderName, "Alice W")
	}
}

// ---------------------------------------------------------------------------
// ParseMessages
// ---------------------------------------------------------------------------

const messagesFixture = `{
	"included": [
		{
			"$type": "com.linkedin.messenger.MessagingParticipant",
			"entityUrn": "urn:li:msg_participant:sender1",
			"hostIdentityUrn": "urn:li:fsd_profile:AAA",
			"participantType": {
				"member": {
					"firstName": {"text": "John"},
					"lastName": {"text": "Smith"}
				}
			}
		},
		{
			"$type": "com.linkedin.messenger.MessagingParticipant",
			"entityUrn": "urn:li:msg_participant:sender2",
			"hostIdentityUrn": "urn:li:fsd_profile:BBB",
			"participantType": {
				"member": {
					"firstName": {"text": "Jane"},
					"lastName": {"text": "Doe"}
				}
			}
		},
		{
			"$type": "com.linkedin.messenger.Message",
			"entityUrn": "urn:li:msg_message:m1",
			"body": {"text": "Hello!"},
			"*sender": "urn:li:msg_participant:sender1",
			"deliveredAt": 1707321600000
		},
		{
			"$type": "com.linkedin.messenger.Message",
			"entityUrn": "urn:li:msg_message:m2",
			"body": {"text": "Hi there, John!"},
			"*sender": "urn:li:msg_participant:sender2",
			"deliveredAt": 1707321700000
		},
		{
			"$type": "com.linkedin.messenger.Message",
			"entityUrn": "urn:li:msg_message:m3",
			"body": {"text": "How have you been?"},
			"*sender": "urn:li:msg_participant:sender1",
			"deliveredAt": 1707321800000
		}
	]
}`

func TestParseMessages(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(messagesFixture), &raw); err != nil {
		t.Fatalf("bad fixture JSON: %v", err)
	}

	msgs := ParseMessages(raw)
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}

	// Sorted chronologically ascending.
	if msgs[0].BodyText != "Hello!" {
		t.Errorf("msgs[0].BodyText = %q", msgs[0].BodyText)
	}
	if msgs[0].SenderName != "John Smith" {
		t.Errorf("msgs[0].SenderName = %q", msgs[0].SenderName)
	}
	if msgs[0].DeliveredAt != 1707321600000 {
		t.Errorf("msgs[0].DeliveredAt = %d", msgs[0].DeliveredAt)
	}

	if msgs[1].BodyText != "Hi there, John!" {
		t.Errorf("msgs[1].BodyText = %q", msgs[1].BodyText)
	}
	if msgs[1].SenderName != "Jane Doe" {
		t.Errorf("msgs[1].SenderName = %q", msgs[1].SenderName)
	}

	if msgs[2].BodyText != "How have you been?" {
		t.Errorf("msgs[2].BodyText = %q", msgs[2].BodyText)
	}
	if msgs[2].SenderName != "John Smith" {
		t.Errorf("msgs[2].SenderName = %q", msgs[2].SenderName)
	}
}

func TestParseMessages_EmptyIncluded(t *testing.T) {
	raw := map[string]any{"included": []any{}}
	msgs := ParseMessages(raw)
	if msgs != nil {
		t.Errorf("expected nil, got %d messages", len(msgs))
	}
}

func TestParseMessages_NoParticipants(t *testing.T) {
	// Messages without matching participants should still parse (SenderName empty).
	fixture := `{
		"included": [
			{
				"$type": "com.linkedin.messenger.Message",
				"entityUrn": "urn:li:msg_message:m1",
				"body": {"text": "Orphan message"},
				"*sender": "urn:li:msg_participant:unknown",
				"deliveredAt": 1700000000000
			}
		]
	}`
	var raw map[string]any
	json.Unmarshal([]byte(fixture), &raw)
	msgs := ParseMessages(raw)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].BodyText != "Orphan message" {
		t.Errorf("BodyText = %q", msgs[0].BodyText)
	}
	if msgs[0].SenderName != "" {
		t.Errorf("SenderName = %q, want empty", msgs[0].SenderName)
	}
	if msgs[0].SenderURN != "urn:li:msg_participant:unknown" {
		t.Errorf("SenderURN = %q", msgs[0].SenderURN)
	}
}

// ---------------------------------------------------------------------------
// FindConversationByProfileURN
// ---------------------------------------------------------------------------

func TestFindConversationByProfileURN(t *testing.T) {
	convos := []Conversation{
		{
			EntityURN: "conv1",
			Participants: []Participant{
				{ProfileURN: "urn:li:fsd_profile:AAA"},
				{ProfileURN: "urn:li:fsd_profile:BBB"},
			},
		},
		{
			EntityURN: "conv2",
			Participants: []Participant{
				{ProfileURN: "urn:li:fsd_profile:AAA"},
				{ProfileURN: "urn:li:fsd_profile:CCC"},
			},
		},
	}

	t.Run("finds matching conversation", func(t *testing.T) {
		c := FindConversationByProfileURN(convos, "urn:li:fsd_profile:CCC")
		if c == nil {
			t.Fatal("expected non-nil")
		}
		if c.EntityURN != "conv2" {
			t.Errorf("EntityURN = %q, want conv2", c.EntityURN)
		}
	})

	t.Run("returns nil for unknown profile", func(t *testing.T) {
		c := FindConversationByProfileURN(convos, "urn:li:fsd_profile:ZZZ")
		if c != nil {
			t.Errorf("expected nil, got %q", c.EntityURN)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		c := FindConversationByProfileURN(nil, "urn:li:fsd_profile:AAA")
		if c != nil {
			t.Error("expected nil for empty list")
		}
	})
}

// ---------------------------------------------------------------------------
// Participant.FullName
// ---------------------------------------------------------------------------

func TestParticipantFullName(t *testing.T) {
	tests := []struct {
		first, last, want string
	}{
		{"Jane", "Doe", "Jane Doe"},
		{"Jane", "", "Jane"},
		{"", "Doe", "Doe"},
		{"", "", ""},
	}
	for _, tt := range tests {
		p := Participant{FirstName: tt.first, LastName: tt.last}
		if got := p.FullName(); got != tt.want {
			t.Errorf("(%q, %q).FullName() = %q, want %q", tt.first, tt.last, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ListConversations (with mock HTTP server)
// ---------------------------------------------------------------------------

func TestListConversations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path includes the messaging GraphQL endpoint.
		if !strings.Contains(r.URL.Path, "voyagerMessagingGraphQL/graphql") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		// Verify variables contain the encoded mailbox URN.
		q := r.URL.RawQuery
		if !strings.Contains(q, "mailboxUrn%3A") {
			// The URN colons should NOT be double-encoded. They appear as %3A in the raw query.
			// But note: the tuple structure uses literal colons. Let me check...
			// Actually the variables use literal colons for tuple syntax, and %3A inside URN values.
			// So the raw query will have: mailboxUrn:urn%3Ali%3Afsd_profile%3AAAA
		}
		if !strings.Contains(q, "queryId="+DefaultConversationsQueryID) {
			t.Errorf("missing queryId in query: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, conversationsFixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "test", JSessionID: "ajax:test"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	li := NewLinkedIn(c)
	convos, err := li.ListConversations(context.Background(), "urn:li:fsd_profile:AAA", 20)
	if err != nil {
		t.Fatalf("ListConversations() error: %v", err)
	}
	if len(convos) != 2 {
		t.Fatalf("got %d conversations, want 2", len(convos))
	}
}

func TestListConversations_EmptyProfileURN(t *testing.T) {
	c, _ := NewClient(auth.Cookies{LiAt: "x", JSessionID: "ajax:y"})
	li := NewLinkedIn(c)
	_, err := li.ListConversations(context.Background(), "", 20)
	if err == nil {
		t.Fatal("expected error for empty profile URN")
	}
}

// ---------------------------------------------------------------------------
// GetMessages (with mock HTTP server)
// ---------------------------------------------------------------------------

func TestGetMessages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "voyagerMessagingGraphQL/graphql") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		q := r.URL.RawQuery
		if !strings.Contains(q, "queryId="+DefaultMessagesQueryID) {
			t.Errorf("missing messages queryId in query: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, messagesFixture)
	}))
	defer ts.Close()

	c, err := NewClient(
		auth.Cookies{LiAt: "test", JSessionID: "ajax:test"},
		WithBaseURL(ts.URL+"/voyager/api"),
	)
	if err != nil {
		t.Fatal(err)
	}

	li := NewLinkedIn(c)
	msgs, err := li.GetMessages(context.Background(), "urn:li:msg_conversation:(urn:li:fsd_profile:AAA,thread001)", 20)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
	// Chronological order
	if msgs[0].BodyText != "Hello!" {
		t.Errorf("msgs[0].BodyText = %q", msgs[0].BodyText)
	}
	if msgs[2].BodyText != "How have you been?" {
		t.Errorf("msgs[2].BodyText = %q", msgs[2].BodyText)
	}
}

func TestGetMessages_EmptyConversationURN(t *testing.T) {
	c, _ := NewClient(auth.Cookies{LiAt: "x", JSessionID: "ajax:y"})
	li := NewLinkedIn(c)
	_, err := li.GetMessages(context.Background(), "  ", 20)
	if err == nil {
		t.Fatal("expected error for empty conversation URN")
	}
}

// ---------------------------------------------------------------------------
// Fixtures are valid JSON
// ---------------------------------------------------------------------------

func TestMessagingFixtures_ValidJSON(t *testing.T) {
	fixtures := map[string]string{
		"conversationsFixture": conversationsFixture,
		"messagesFixture":      messagesFixture,
	}
	for name, f := range fixtures {
		var v any
		if err := json.Unmarshal([]byte(f), &v); err != nil {
			t.Errorf("fixture %s is not valid JSON: %v", name, err)
		}
	}
}
