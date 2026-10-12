# WebSocket Event Contract

Status: Phase 0 proposal · Protocol: `mc.v1` · Related: [REALTIME.md](../architecture/REALTIME.md), [API-DESIGN.md](API-DESIGN.md)

The WebSocket carries **server events** (things that happened) and a small set of **client commands** (ephemeral signals such as typing). Durable writes always use REST.

---

## 1. Endpoint and handshake

| Step | Detail |
|---|---|
| 1. Ticket | `POST /api/v1/orgs/{orgId}/realtime/tickets` (session cookie + `X-CSRF-Token`) → `{ "ticket": "…", "url": "/api/v1/realtime" }`. Single use, 30 s, bound to user + session + organisation. |
| 2. Connect | `new WebSocket("wss://<host>/api/v1/realtime?ticket=<ticket>", ["mc.v1"])` — same origin, cookie sent automatically. |
| 3. Server checks | **Before upgrade:** `Origin` must be in the allow-list, otherwise HTTP 403 and no socket (the browser only sees a generic failure; legitimate clients never hit this). **After upgrade:** session cookie valid → ticket valid and matching the session → subprotocol supported; failures close immediately with 4001 or 4010 so the client can react to the code. |
| 4. Hello | First client frame must be `hello` within 10 s. |
| 5. Welcome | Server replies `welcome`; implicit subscriptions are active from this point. |

## 2. Frames

* UTF-8 JSON text frames, one message per frame. Binary frames are rejected.
* Inbound (client → server) frames ≤ 16 KB. Outbound events ≤ 64 KB (larger content is fetched over REST; see `truncated`).
* Heartbeat: the server sends WebSocket ping control frames every 25 s (browsers answer automatically). Because browsers cannot observe control frames, the client also sends an application `ping` after 30 s without inbound traffic and treats a missing `pong` within 10 s as a dead connection.

### 2.1 Server event envelope

```json
{
  "v": 1,
  "id": "0199b2c4-8c30-7f12-a3c1-5b0e7d9f1c22",
  "type": "message.created",
  "ts": "2026-10-12T10:04:11.502Z",
  "org_id": "0199a6f0-1d2e-7c11-9a55-0b3c4d5e6f70",
  "conversation_id": "0199a6f2-5b1c-7e44-8a09-6c2d1e3f4a50",
  "change_seq": 5187,
  "data": { }
}
```

| Field | Meaning |
|---|---|
| `v` | Envelope version (1) |
| `id` | Unique event ID (UUIDv7) — deduplicate on it |
| `type` | Event name, `resource.action` (`message.reaction.added`) |
| `ts` | Server time the event was produced (post-commit) |
| `org_id` | Organisation the event belongs to |
| `conversation_id`, `change_seq` | Present on every conversation-scoped **message** event (created, updated, deleted, reactions, pins): the conversation's change-feed position after this change. A gap (`change_seq` > last seen + 1) means events were missed → `GET …/conversations/{id}/changes?after=` ([REALTIME.md §6](../architecture/REALTIME.md#6-client-convergence)) |
| `data` | Event payload (schema per type). Messages inside carry their own `seq` (top-level) or `thread_seq` (reply) for placement |

### 2.2 Client command envelope and replies

```json
{ "v": 1, "id": "c-17", "type": "subscribe", "data": { "topics": [ { "kind": "conversation", "id": "0199a6f3-…" } ] } }
```

```json
{ "v": 1, "type": "ack",   "re": "c-17", "data": { "subscribed": ["conversation:0199a6f3-…"] } }
{ "v": 1, "type": "error", "re": "c-17", "data": { "code": "forbidden", "message": "You cannot view this conversation." } }
```

`id` is chosen by the client (unique per connection); `re` echoes it.

## 3. Versioning rules

1. **Protocol version** = WebSocket subprotocol (`mc.v1`). A future `mc.v2` is offered alongside `mc.v1` for at least 90 days; the client lists the versions it supports and the server picks the newest common one.
2. **Within a protocol version, changes are additive:** new event types, new optional fields, new enum values. Clients **must ignore** unknown event types and unknown fields, and treat unknown enum values as "other".
3. **Breaking change to one event** → a new type name with a version suffix (e.g. `message.created.v2`), sent only to clients that list it in `hello.capabilities`; the old type continues until the deprecation window ends.
4. **Envelope changes** require a new protocol version.
5. Every event type has a JSON Schema in `packages/contracts/events/` (draft 2020-12 — the same dialect as the OpenAPI 3.1 documents, so event payloads `$ref` the REST component schemas such as `Message` instead of duplicating them). TypeScript types are generated from them; Go emitters are tested against them (see [TESTING.md](../development/TESTING.md#realtime-and-websocket-tests)).

## 4. Close codes

| Code | Meaning | Client behaviour |
|---|---|---|
| 1000 / 1001 | Normal / going away | Reconnect only if still needed |
| 1011 | Server error | Reconnect with backoff |
| 1012 | Service restart (drain) | Reconnect after `retry_after_ms` from the preceding `reconnect` event |
| 4001 | Unauthenticated (ticket or session invalid), sent right after the upgrade | Fetch session; if signed out, go to login; else new ticket |
| — (HTTP 403, seen as 1006) | Origin not allowed — refused before the upgrade | Treat repeated 1006 failures on connect as fatal and surface a support error |
| 4004 | Organisation access lost (membership deactivated/removed, organisation suspended) | Leave the organisation view; other organisations unaffected |
| 4008 | Slow consumer | Reconnect and resync |
| 4009 | Session revoked | Sign out locally |
| 4010 | Unsupported protocol | Reload the application |
| 4029 | Rate limited | Back off ≥ 30 s |

Reconnect backoff: 0.5 s, 1 s, 2 s, 4 s … capped at 30 s, ±30 % jitter; immediate attempt when the browser reports `online`.

## 5. Client commands

| Type | Data | Notes |
|---|---|---|
| `hello` | `{ "client": "web/1.0.0", "capabilities": ["message.created"], "resume": { "last_event_at": "…" } }` | Required first frame |
| `ping` | `{}` | Server replies `pong` |
| `subscribe` | `{ "topics": [ { "kind": "conversation" \| "meeting", "id": "…" } ] }` | Explicit topics: public channel preview, meeting roster/waiting room. Authorised like the REST read. Max 50 |
| `unsubscribe` | `{ "topics": [ … ] }` | |
| `typing` | `{ "conversation_id": "…", "thread_root_id": "…" \| null }` | ≤ 1 per 3 s per conversation; requires membership |
| `presence.subscribe` | `{ "user_ids": ["…"] }` | Replaces the previous set; ≤ 500 IDs; filtered by visibility |
| `presence.idle` / `presence.active` | `{}` | Idle after 10 min without input |

## 6. Server events

Audience legend: **conv** = conversation members (+ explicit subscribers for public channels), **space** = Space members, **org** = organisation members (guests filtered), **user** = the user's own connections, **hosts** = meeting host/co-hosts, **meeting** = meeting topic subscribers.

### 6.1 Connection

| Type | Audience | Data |
|---|---|---|
| `welcome` | connection | `connection_id`, `protocol`, `heartbeat_ms`, `server_time`, `user_id`, `org_id` |
| `pong` | connection | `{}` |
| `reconnect` | connection | `retry_after_ms`, `reason` (`drain`, `rebalance`) |
| `session.revoked` | user | `reason` (`signed_out`, `password_changed`, `password_reset`, `account_disabled`) — followed by close 4009 |
| `realtime.degraded` / `realtime.recovered` | connection | `poll_interval_ms` |

### 6.2 Organisation, users and Spaces

| Type | Audience | Data |
|---|---|---|
| `org.updated` | org | organisation summary |
| `org.settings.updated` | org | `section`, `version` (clients refetch if relevant) |
| `org.member.updated` | org | `user_id`, `role`, `status`, `title`, `status_text` (per-organisation status) |
| `org.member.removed` | org | `user_id` |
| `user.updated` | org | `user_id`, `display_name`, `avatar_version` (global identity fields only) |
| `space.created` / `space.updated` | space (public Spaces: non-guest org members) | Space summary |
| `space.archived` / `space.unarchived` / `space.deleted` | space | `space_id` |
| `space.member.added` / `.updated` / `.removed` | space + affected user | `space_id`, `user_id`, `role` |

### 6.3 Conversations

| Type | Audience | Data |
|---|---|---|
| `conversation.created` / `conversation.updated` | conv (public channel creation: space) | conversation summary |
| `conversation.archived` / `conversation.unarchived` | conv | `conversation_id` |
| `conversation.member.joined` / `conversation.member.left` | conv + affected user | `conversation_id`, `user_id` |
| `conversation.read` | user | `conversation_id`, `last_read_seq`; or `thread_root_id`, `last_read_thread_seq` for threads |
| `conversation.member.read` | conv (DMs and groups ≤ 10, receipts enabled) | `conversation_id`, `user_id`, `last_read_seq` |
| `conversation.preferences.updated` | user | `conversation_id`, `is_favorite`, `hidden`, `notification_level`, `muted_until` |

### 6.4 Messages

All rows except `typing` carry `conversation_id` and `change_seq` in the envelope.

| Type | Audience | Data |
|---|---|---|
| `message.created` | conv | `message` (as REST: `seq` for top-level or `thread_seq` for replies), `thread_root` summary `{id, reply_count, last_reply_seq, last_reply_at}` for replies, `truncated` |
| `message.updated` | conv | `message` (body, mentions, `edited_at`, `version`) |
| `message.deleted` | conv | `message_id`, `thread_root_id`, `version` |
| `message.reaction.added` / `message.reaction.removed` | conv | `message_id`, `emoji`, `user_id` (actor), `count` (new total for that emoji — a snapshot, not a delta) |
| `message.pinned` / `message.unpinned` | conv | `message_id`, `by_user_id` |
| `typing` | conv | `conversation_id`, `thread_root_id`, `user_id`, `expires_in_ms` (ephemeral; no `change_seq`) |

### 6.5 Presence and files

| Type | Audience | Data |
|---|---|---|
| `presence.updated` | presence subscribers in the same organisation | `user_id`, `status` (`available`, `away`, `busy`, `dnd`, `offline`), `status_text`, `status_expires_at` — all per organisation |
| `file.updated` | uploader + conv | `file_id`, `status`, `preview_available`, `conversation_id` |
| `file.deleted` | uploader + conv | `file_id` |

### 6.6 Meetings, huddles and calls

| Type | Audience | Data |
|---|---|---|
| `meeting.created` / `meeting.updated` / `meeting.cancelled` | invitees + entitled members | meeting summary |
| `meeting.started` / `meeting.ended` | invitees + meeting | `meeting_id`, `kind`, `started_at` / `ended_at`, `duration_s` |
| `meeting.participant.joined` / `meeting.participant.left` | meeting (+ channel members for huddles) | `meeting_id`, `user_id` |
| `meeting.waiting_room.updated` | hosts | `meeting_id`, `waiting: [{user_id, requested_at}]` |
| `meeting.admitted` / `meeting.denied` | user | `meeting_id` |
| `huddle.updated` | conv (channel) | `conversation_id`, `meeting_id`, `status`, `participant_ids` |
| `call.incoming` | callees | `meeting_id`, `conversation_id`, `from_user_id`, `mode` (`audio`/`video`), `expires_at` |
| `call.answered` / `call.declined` / `call.cancelled` / `call.missed` | caller + callees | `meeting_id`, `user_id` |

### 6.7 Notifications, actions, support, integrations

| Type | Audience | Data |
|---|---|---|
| `notification.created` | user | notification (as REST Activity item) |
| `notification.read` | user | `ids` or `all: true` |
| `unread.updated` | user | `conversations` (map id → count, capped), `mentions`, `activity` |
| `action.created` / `action.updated` / `action.deleted` | creator + assignee | action item |
| `support_request.updated` | requester | `id`, `status`, `delivery_status`, `external_url` |
| `support_request.message.created` | requester | `support_request_id`, `message` |
| `integration.updated` | org Owners/Admins | `id`, `status`, `last_test_ok` |

## 7. Examples

**welcome**

```json
{ "v": 1, "id": "0199b2c0-…", "type": "welcome", "ts": "2026-10-12T09:59:58.010Z", "org_id": "0199a6f0-…",
  "data": { "connection_id": "c_8f2a…", "protocol": "mc.v1", "heartbeat_ms": 25000,
            "server_time": "2026-10-12T09:59:58.010Z", "user_id": "0199a6f1-…" } }
```

**message.created (thread reply)**

```json
{ "v": 1, "id": "0199b2c4-8c30-…", "type": "message.created", "ts": "2026-10-12T10:08:02.144Z",
  "org_id": "0199a6f0-…", "conversation_id": "0199a6f2-…", "change_seq": 5188,
  "data": {
    "message": { "id": "0199b2c5-…", "conversation_id": "0199a6f2-…", "seq": null, "thread_seq": 1,
                 "thread_root_id": "0199b2c4-8c22-…", "client_msg_id": "0199b2c5-0000-…",
                 "sender": { "id": "0199a6f1-cccc-…", "display_name": "Daniel Chen" },
                 "body": "Infrastructure checks passed on my side.", "mentions": [], "attachments": [],
                 "reactions": [], "reply_count": 0, "version": 1,
                 "created_at": "2026-10-12T10:08:02.120Z", "edited_at": null },
    "thread_root": { "id": "0199b2c4-8c22-…", "reply_count": 1, "last_reply_seq": 1, "last_reply_at": "2026-10-12T10:08:02.120Z" },
    "truncated": false } }
```

**typing**

```json
{ "v": 1, "id": "0199b2c6-…", "type": "typing", "ts": "2026-10-12T10:08:30.002Z", "org_id": "0199a6f0-…",
  "data": { "conversation_id": "0199a6f2-…", "thread_root_id": null, "user_id": "0199a6f1-dddd-…", "expires_in_ms": 6000 } }
```

**call.incoming**

```json
{ "v": 1, "id": "0199b2d1-…", "type": "call.incoming", "ts": "2026-10-12T10:20:00.000Z", "org_id": "0199a6f0-…",
  "data": { "meeting_id": "0199b2d0-…", "conversation_id": "0199a6f9-…", "from_user_id": "0199a6f1-bbbb-…",
            "mode": "video", "expires_at": "2026-10-12T10:20:45.000Z" } }
```

**error reply**

```json
{ "v": 1, "type": "error", "re": "c-21", "data": { "code": "rate_limited", "message": "Slow down.", "retry_after_ms": 3000 } }
```

## 8. Privacy and security rules

* An event is delivered only to connections whose user could fetch the same data over REST at that moment.
* Guests never receive events about Spaces, conversations or people outside their shared Spaces; email addresses never appear in events.
* Message bodies appear only in `message.*` events to conversation audiences — never in `notification.*`, presence or typing events.
* Read receipts (`conversation.member.read`) respect both users' privacy settings.
* Servers rate-limit and validate every command; repeated violations close the socket with 4029 or 1008.
