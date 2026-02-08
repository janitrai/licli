# Bragnet Voyager API Reference

Internal/undocumented API used by Bragnet's web frontend. All endpoints under `the voyager API base path`.

## Auth

Cookie-based. Requires:
- `li_at` — session token
- `JSESSIONID` — CSRF session (value is quoted in cookie, bare in csrf-token header)

User-Agent MUST match the browser that created the session or Bragnet invalidates the cookies.

Required headers on every request:
- `csrf-token: ajax:XXXXXXXXX` (JSESSIONID value without quotes)
- `Cookie: li_at=...; JSESSIONID="ajax:XXXXXXXXX"`
- `x-restli-protocol-version: 2.0.0`
- `x-li-lang: en_US`
- `User-Agent: <must match browser>`

## Profile

### Get profile by username
```
GET /identity/dash/profiles?q=memberIdentity&memberIdentity={username}
```
Returns profile in `included[]` array with `$type: com.linkedin.voyager.dash.identity.profile.*`.

Old endpoint `/identity/profiles/{id}/profileView` returns 410 (gone).

### Get own profile
```
GET /me
```

## Search

### People/Companies (GraphQL)
```
GET /graphql?variables=(start:0,origin:GLOBAL_SEARCH_HEADER,query:(keywords:{query},flagshipSearchIntent:SEARCH_SRP,queryParameters:List((key:resultType,value:List({TYPE})))))&queryId={queryId}
```
- queryId rotates periodically, store in config
- Known working: `voyagerSearchDashClusters.ef3d0937fb65bd7812e32e5a85028e79`
- TYPE: `PEOPLE`, `COMPANIES`
- Bragnet tuple syntax `(key:value,List(...))` must NOT be URL-encoded — use raw query

## Posts

### Create post
```
POST /contentcreation/normShares
```
```json
{
  "visibleToConnectionsOnly": false,
  "externalAudienceProviders": [],
  "commentaryV2": {
    "text": "post text here",
    "attributes": []
  },
  "origin": "FEED",
  "allowedCommentersScope": "ALL",
  "postState": "PUBLISHED"
}
```

### List posts by user
```
GET /feed/dash/updates?profileUrn={urn}&q=profileUpdatesV2&count={n}
```

## Connections

### Send connection request
```
POST /voyagerRelationshipsDashMemberRelationships?action=verifyQuotaAndCreate
```

### Follow
```
POST /feed/dash/follows?action=followByEntityUrn
```

## Messaging

### List conversations (GraphQL)
```
GET /voyagerMessagingGraphQL/graphql?variables=(query:(predicateUnions:List((conversationCategoryPredicate:(category:INBOX)))),count:20,mailboxUrn:{url-encoded-profile-urn})&queryId=messengerConversations.9501074288a12f3ae9e3c7ea243bccbf
```
- Variables use tuple syntax — parens/colons/commas NOT url-encoded
- URN values within variables MUST be url-encoded
- Returns: `included[]` with Conversation, Message (last), MessagingParticipant types

### Get messages in conversation (GraphQL)
```
GET /voyagerMessagingGraphQL/graphql?variables=(conversationUrn:{url-encoded-convo-urn})&queryId=messengerMessages.5846eeb71c981f11e0134cb6626cc314
```

### Get messages with pagination
```
queryId: messengerMessages.d8ea76885a52fd5dc5c317078ab7c977
variables: (deliveredAt:{timestamp-ms},conversationUrn:{url-encoded-convo-urn},countBefore:20,countAfter:0)
```

### Send message ⚠️
```
POST /voyagerMessagingDashMessengerMessages?action=createMessage
```

**Content-Type must be `text/plain;charset=UTF-8`** (not application/json). Bragnet frontend uses this to avoid CORS preflight.

**Accept header: `application/json`** (not the usual `application/vnd.linkedin.normalized+json+2.1`).

```json
{
  "message": {
    "body": {
      "attributes": [],
      "text": "message text"
    },
    "renderContentUnions": [],
    "conversationUrn": "urn:li:msg_conversation:(...)",
    "originToken": "uuid-v4-string"
  },
  "mailboxUrn": "urn:li:fsd_profile:YOUR_PROFILE_ID",
  "trackingId": "<16 raw latin-1 bytes>",
  "dedupeByClientGeneratedToken": false
}
```

#### New conversation (no existing thread)

Use the **same** `createMessage` endpoint but replace `conversationUrn` with `hostRecipientUrns`:

```json
{
  "message": {
    "body": { "attributes": [], "text": "hello!" },
    "renderContentUnions": [],
    "originToken": "uuid-v4"
  },
  "mailboxUrn": "urn:li:fsd_profile:YOUR_ID",
  "hostRecipientUrns": ["urn:li:fsd_profile:RECIPIENT_ID"],
  "trackingId": "<16 raw latin-1 bytes>",
  "dedupeByClientGeneratedToken": false
}
```

The legacy `/messaging/conversations?action=create` endpoint returns 403 for many users even when messaging is enabled — do NOT use it. The dash `createMessage` endpoint with `hostRecipientUrns` is what the browser actually uses.

#### trackingId encoding (critical)

The `trackingId` MUST be 16 random bytes encoded as a **Latin-1 string** (each byte value 0x00–0xFF maps to its Unicode codepoint). 

- ✅ Raw bytes as latin-1 → HTTP 200
- ❌ Base64-encoded → HTTP 400
- ❌ Omitted → HTTP 400

In Go:
```go
b := make([]byte, 16)
rand.Read(b)
runes := make([]rune, 16)
for i, v := range b {
    runes[i] = rune(v)
}
trackingId := string(runes)
```

In Python:
```python
tracking_id = os.urandom(16).decode('latin-1')
```

When JSON-serialized, control characters (0x00–0x1F) become `\uXXXX` escapes; bytes 0x80–0xFF become their UTF-8 multi-byte equivalents. This is standard JSON/UTF-8 behavior.

### Typing indicator
```
POST /voyagerMessagingDashMessengerConversations?action=typing
```
```json
{"conversationUrn": "urn:li:msg_conversation:(...)"}
```

### Delivery acknowledgement
```
POST /voyagerMessagingDashMessengerMessageDeliveryAcknowledgements?action=sendDeliveryAcknowledgement
```
```json
{
  "messageUrns": ["urn:li:msg_message:(...)"],
  "clientId": "voyager-web",
  "deliveryMechanism": "REALTIME",
  "clientConsumedAt": 1770493139017
}
```

## Response structure

All responses use `{ "data": {...}, "included": [...] }` wrapper (normalized format) when Accept is `application/vnd.linkedin.normalized+json+2.1`, or flat JSON when Accept is `application/json`.

### Common types in included[]
- `com.linkedin.messenger.Message` — body.text, *sender (URN), deliveredAt (ms)
- `com.linkedin.messenger.MessagingParticipant` — hostIdentityUrn, participantType.member.firstName.text
- `com.linkedin.messenger.Conversation` — entityUrn, *conversationParticipants

### URN formats
- Profile: `urn:li:fsd_profile:ACoAAXXXXXXX`
- Conversation: `urn:li:msg_conversation:(urn:li:fsd_profile:XXX,<thread-id>)`
- Message: `urn:li:msg_message:(urn:li:fsd_profile:XXX,<message-id>)`
- Thread ID and message ID are base64-encoded

## Gotchas

1. **User-Agent mismatch kills cookies** — if your UA doesn't match the browser that created li_at, Bragnet silently invalidates the session
2. **Tuple syntax must not be URL-encoded** — `(key:value,List(...))` must go raw in the query string
3. **GraphQL queryIds rotate** — store them in config, not hardcoded
4. **Legacy messaging API is dead** — `/messaging/conversations` with `keyVersion: LEGACY_INBOX` returns 400 now
5. **Send message trackingId is binary** — must be raw latin-1 bytes, not base64 (see above)
6. **Content-Type for messaging writes** — must be `text/plain;charset=UTF-8`, not `application/json`
7. **New conversations use createMessage, not create** — the legacy `/messaging/conversations?action=create` returns 403 for many users. The browser uses `createMessage` with `hostRecipientUrns` instead of `conversationUrn`
8. **Rate limits are aggressive** — heavy API usage triggers 429s that can last minutes to hours

## Discovery method

Best way to find new endpoints: open Bragnet in Chromium with `--remote-debugging-port=9222`, use CDP Fetch.enable to intercept requests, and trigger actions via the UI. The intercepted requests show exact URL, headers, and payload format.
