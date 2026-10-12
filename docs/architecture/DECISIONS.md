# Architecture Decision Record

Status legend: **Accepted** (approved for implementation) · **Proposed** (recommended refinement, low risk, adopted unless implementation shows otherwise).

Phase 0 was approved on 2026-10-12, including ADR-004, ADR-008, ADR-010, ADR-011, ADR-020 and ADR-022 and the guest policy recorded in ADR-025.

Each record: context → decision → consequences. New decisions are appended; superseded ones are marked, never deleted.

---

## Summary

| ID | Decision | Status |
|---|---|---|
| ADR-001 | Modular monolith: one Go module, binaries `api`, `worker`, `mcctl` | Accepted (refined layout) |
| ADR-002 | Organisation in every API path; isolation via composite FKs + PostgreSQL RLS | Proposed |
| ADR-003 | PostgreSQL 18, UUIDv7 keys, goose migrations, sqlc + pgx | Proposed |
| ADR-004 | Background jobs on PostgreSQL (River) instead of Redis Streams | **Accepted** |
| ADR-005 | Realtime: post-commit Redis pub/sub; `seq`/`thread_seq` for placement, per-conversation `change_seq` change feed for convergence | Proposed |
| ADR-006 | Channels are conversations; read state as cursors; Activity derived from notifications | Proposed |
| ADR-007 | Next.js renders UI only; the Go API is the only backend; one origin via Nginx | Proposed |
| ADR-008 | Product accent switches from prototype blue to Library Violet/Navy | **Accepted** |
| ADR-009 | Inter (self-hosted, OFL) for UI text; HemiHead only via SVG lockups | Proposed |
| ADR-010 | S3 API everywhere; MinIO for local dev only, pinned; production storage chosen separately | **Accepted** |
| ADR-011 | Redis-compatible server: Valkey 8 through the Redis protocol | **Accepted** |
| ADR-012 | WebSocket: `coder/websocket`, cookie + single-use ticket + Origin check, subprotocol `mc.v1` | Proposed |
| ADR-013 | REST `/api/v1`, OpenAPI contract-first, Problem Details errors, cursor pagination | Proposed |
| ADR-014 | Server-side sessions, Argon2id, TOTP + recovery codes, CSRF header; no JWT browser sessions | Accepted |
| ADR-015 | Malware scanning with ClamAV behind a `Scanner` interface; fail closed in production | Proposed |
| ADR-016 | Root task runner = pnpm scripts (no `make` dependency) | Proposed |
| ADR-017 | LiveKit self-hosted; explicit room creation; waiting room by deferring tokens; identity = user ID | Proposed |
| ADR-018 | Search on PostgreSQL FTS (`simple` config) + `pg_trgm`, behind a swappable interface | Proposed |
| ADR-019 | Support context = immutable snapshot of explicitly selected items | Proposed |
| ADR-020 | Email notifications exclude message content by default | **Accepted** |
| ADR-021 | Phase sequence adjustments (mailer in Phase 2, security foundations early) | Proposed |
| ADR-022 | V1 ships the HelpDesk connector architecture + SMTP connector; MateDesk native connector when its API exists | **Accepted** |
| ADR-023 | Presence, status text and the visible email address are per organisation | Proposed |
| ADR-024 | Target Go 1.27 (Go 1.25 is out of security support) | **Accepted** |
| ADR-025 | Guest policy: granted Spaces; invited meetings when allowed; guest DMs off by default | **Accepted** |

---

## ADR-001 — Modular monolith in a single Go module

**Context.** The brief asks for a modular monolith with `services/api` and `services/worker`. The worker needs the same domain services (notifications, support delivery, file processing). Go's `internal/` visibility rule prevents sharing packages across separate Go modules.

**Decision.** One Go module (Go 1.27, see ADR-024) rooted at `services/` with three binaries — `services/api`, `services/worker`, `services/mcctl` (admin CLI) — sharing `services/internal/...`. Business logic lives in module services; handlers are thin; repositories isolate SQL.

**Consequences.** Shared code without publishing packages; one dependency graph; module boundaries enforced by convention, review and an import-lint test (Phase 1). The directory names from the brief are preserved.

## ADR-002 — Tenant isolation in depth

**Context.** "Never design a system where a missing WHERE clause can casually expose another organisation's data." Application-only checks fail open when a query is wrong.

**Decision.** (1) Organisation ID in every tenant API path (`/api/v1/orgs/{orgId}/…`); (2) membership resolved into an `Actor` by middleware; (3) repository functions require a typed `OrgID`; (4) every tenant table has `org_id NOT NULL`; (5) **composite foreign keys** `(org_id, x_id)` so rows can never reference another tenant's rows; (6) **PostgreSQL row-level security** on tenant tables keyed to a transaction-local `mc.org_id` setting, with the application connecting as a non-owner role; (7) namespacing of Redis keys, pub/sub channels, object keys and LiveKit rooms; (8) automated cross-tenant tests for every endpoint.

**Consequences.** Some query and migration complexity; every tenant query runs inside a transaction that sets tenant context; a small set of pre-context lookups (login, invitation token) use narrow `SECURITY DEFINER` functions. Details: [MULTITENANCY.md](MULTITENANCY.md).

## ADR-003 — PostgreSQL 18, UUIDv7, goose, sqlc + pgx

**Decision.** PostgreSQL 18 (native `uuidv7()`); UUIDv7 primary keys generated in Go (time-ordered, index-friendly, safe to expose); goose SQL migrations in `db/migrations` executed by `mcctl migrate` as a separate deployment step; sqlc generates typed Go from `db/queries/**/*.sql`; pgx v5 driver and pool. Enumerations are `text` + `CHECK` (cheaper to evolve than `ENUM` types).

**Consequences.** No ORM magic; migrations reviewable as SQL; sqlc catches query/schema drift at build time.

## ADR-004 — Background jobs on PostgreSQL (River) instead of Redis Streams — *accepted*

**Context.** The brief suggests "Redis Streams or an equivalently simple Redis-backed approach". Background work (notification fanout, emails, malware scans, HelpDesk delivery, reminders, retention) must not be lost and must be triggered exactly when the originating transaction commits. With Redis Streams we would also need an outbox table, a relay process, consumer-group reclaim logic, a dead-letter path and a separate scheduler for delayed jobs.

**Decision.** Use **River** (Go, PostgreSQL-backed, MPL-2.0): jobs are inserted **in the same transaction** as the domain change, so there is no dual-write problem and no outbox relay. River provides retries with backoff, scheduled jobs, unique jobs, periodic (cron) jobs and job inspection. Redis stays purely ephemeral (pub/sub, presence, caches, limits) and can be flushed without losing work. Job code depends on a small `jobs.Enqueuer` interface so the engine can be replaced.

**Consequences.** Job tables add write load to PostgreSQL (acceptable at V1 scale; queues can move to a dedicated database later). River is pre-1.0 — pin the version and keep the abstraction thin. **Fallback if rejected:** transactional `outbox_events` table + relay publishing to Redis Streams consumer groups + a sorted-set scheduler; this adds roughly one phase-worth of infrastructure work.

## ADR-005 — Realtime delivery without an outbox

**Decision.** After commit, the service publishes the event to the local hub and to Redis pub/sub (`mc:rt:org:{orgId}`, `mc:rt:user:{userId}`). Three gap-free sequences separate concerns: `seq` places top-level messages, `thread_seq` places replies, and a per-conversation `change_seq` — advanced by every message-level change (new message, reply, edit, deletion, reaction, pin) — drives convergence: clients detect any missed event as a `change_seq` gap and fetch exactly what changed from the change feed, on gaps, reconnects, focus and a 60 s periodic sync. Durable side effects use River (ADR-004), not the realtime path.

**Consequences.** Simple and fast; a crash between commit and publish delays delivery until the next event, reconnect or focus resync (bounded by the 60 s periodic sync). Edits, deletions and reactions on old messages converge as reliably as new messages. Every message-level write takes the conversation row lock, which serialises writes per conversation (ample for chat workloads). Details: [REALTIME.md](REALTIME.md), [DATABASE.md §4.4](DATABASE.md#44-messages).

## ADR-006 — Normalised conversation model

**Decision.** DMs, group chats, channels and meeting chats are rows in one `conversations` table (`kind`), with one `conversation_members` table. The brief's `channels`/`channel_members` are therefore not separate tables. `message_reads` is replaced by read cursors (`conversation_members.last_read_seq`, `thread_subscriptions.last_read_thread_seq`); DM "Seen" receipts are derived from cursors. `attachments` becomes the `message_attachments` join table over `files`. Activity is a view over `notifications`. `actions` is named `action_items` to avoid ambiguity with permission and audit "actions".

**Consequences.** One messaging pipeline for every conversation kind; O(1) read-state writes instead of O(messages × readers). Details: [DATABASE.md](DATABASE.md#2-normalisation-decisions).

## ADR-007 — Next.js is UI only

**Decision.** Next.js renders pages and the app shell. All data goes browser → Nginx → Go API on the **same origin** (`/api/v1`). No Next.js route handlers or server actions implement business logic, and the Next.js server holds no user credentials. Next.js `proxy.ts` (Next 16) only sets CSP nonces/security headers and performs a cookie-presence redirect for signed-out users.

**Consequences.** One backend to secure and test; no CORS; cookies stay `SameSite=Lax` and `HttpOnly`.

## ADR-008 — Brand accent: Violet replaces prototype blue — *accepted*

**Context.** The prototype's primary colour (`#2463EB`) and logo conflict with the Logo Library (Violet `#7047D1`, Navy `#101A2D`). The brief says the Library wins on branding conflicts.

**Decision.** Production tokens use Violet as the brand/interactive accent (buttons, active navigation, focus rings, links, selected states) and Navy for the rail and brand surfaces. Layout, spacing, radii, neutrals and component shapes stay as the prototype defines them. Contrast has been validated (Violet on white 5.98:1; white on Violet 5.98:1; dark-theme accent `#A48AF0` on `#171E29` 5.96:1).

**Consequences.** The product will look Violet rather than blue. Approval needed because it changes the approved visual direction; the alternative (blue UI with Violet logo only) produces an inconsistent identity.

## ADR-009 — Typography

**Decision.** Inter (variable, SIL OFL) self-hosted through `next/font`, with `system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif` fallback. HemiHead appears only inside Library SVG lockups; the font file is never shipped.

**Consequences.** Consistent rendering across platforms (the prototype silently fell back to Arial on macOS). No font-licensing exposure.

## ADR-010 — Object storage — *accepted*

**Context.** The brief names MinIO for local development. The upstream `minio/minio` repository was archived on 25 April 2026 and marked "no longer maintained"; community container images stopped being published in October 2025.

**Decision.** Code talks only to the S3 API (AWS SDK for Go v2, path-style addressing configurable). Local development keeps MinIO, bound to localhost only. *Implementation:* no community image is published any more, so `infra/docker/minio.Dockerfile` builds the final community source release (`RELEASE.2025-10-15T17-29-55Z`, tag verified against its commit) locally. Production storage is selected separately: a managed S3-compatible service (AWS S3, Cloudflare R2, Backblaze B2, Wasabi) or a maintained self-hosted alternative (e.g. SeaweedFS, Garage, Ceph RGW). Bucket creation and CORS are applied by `mcctl storage init`, not by MinIO-specific tooling.

**Consequences.** No vendor lock-in; local dev unchanged. Decision needed: approve MinIO-for-dev-only and choose the production storage provider before Phase 12.

## ADR-011 — Redis-compatible server — *accepted*

**Context.** Redis 7.4+ is RSALv2/SSPLv1, and Redis 8 adds AGPLv3 as an option; Valkey (Linux Foundation fork of Redis 7.2) remains BSD-licensed and protocol-compatible. MateCollab uses only core features (pub/sub, strings with TTL, sets, sorted sets, Lua for rate limiting).

**Decision.** Recommend **Valkey 8** for all environments; the code uses the Redis protocol via `go-redis` and stays compatible with Redis 8 and managed offerings.

**Consequences.** Avoids AGPL/SSPL review for self-hosted deployments. LiveKit multi-node compatibility with Valkey must be verified in Phase 7 (single-node LiveKit does not need Redis).

## ADR-012 — WebSocket gateway

**Decision.** `github.com/coder/websocket` inside the API binary at `/api/v1/realtime`. Connection requires the session cookie, an allowed `Origin`, and a single-use 30-second ticket from `POST /api/v1/orgs/{orgId}/realtime/tickets` (binds the socket to one organisation). Subprotocol `mc.v1` carries the protocol version. Clients send only lightweight commands (subscribe, typing, presence); writes such as sending messages use REST.

**Consequences.** Defence in depth against cross-site WebSocket hijacking; one write path for messages (REST) with idempotency and rate limiting. Details: [WEBSOCKET-EVENTS.md](../api/WEBSOCKET-EVENTS.md).

## ADR-013 — REST conventions

**Decision.** `/api/v1` URL versioning; JSON with `snake_case`; RFC 9457 Problem Details errors with stable `code` values; opaque cursor pagination (sequence-based for messages); `Idempotency-Key` on side-effecting POSTs; `ETag`/`If-Match` on admin settings; OpenAPI 3.1 source in `packages/contracts` (JSON Schema 2020-12, shared with the WebSocket event schemas), generated TypeScript client types, Go response contract tests. Go tooling compatibility with 3.1 is confirmed in Phase 1.

## ADR-014 — Authentication

**Decision.** Opaque 256-bit session tokens in a `__Host-mc_session` cookie (`Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`), stored hashed (SHA-256) in PostgreSQL with a short Redis cache; Argon2id password hashing (PHC strings, rehash on login when parameters change); TOTP (RFC 6238) with encrypted secrets and replay protection; 10 single-use recovery codes stored as keyed hashes; CSRF via `Origin`/`Sec-Fetch-Site` checks plus a per-session `X-CSRF-Token` header; no JWTs for browser sessions. SSO (OIDC/SAML) is a post-V1 extension point.

## ADR-015 — Malware scanning

**Decision.** ClamAV (`clamd`, INSTREAM over TCP) behind a `Scanner` interface. Production fails closed (files stay unavailable until scanned clean). Local development may use a `noop` scanner, which marks files "not scanned" visibly in the UI.

## ADR-016 — Task runner

**Decision.** Root `package.json` scripts (via pnpm) are the single entry point (`pnpm dev`, `pnpm test`, `pnpm db:migrate` …) and call Go tooling where needed. Rationale: Node and pnpm are already required and cross-platform; `make` is not installed on the Windows development machine.

## ADR-017 — LiveKit integration

**Decision.** Self-hosted LiveKit server; MateCollab owns meetings, permissions, scheduling, chat and history. Rooms are created explicitly by the API (no auto-create) with opaque names (`m_<meetingId>`); tokens are issued only after authorization, live 10 minutes, and carry minimal grants. Waiting rooms are implemented by withholding tokens until a host admits the participant. LiveKit identity = MateCollab user ID (joining from a second device replaces the first). Webhooks are signature-verified and processed idempotently. Details: [MEDIA.md](MEDIA.md).

## ADR-018 — Search

**Decision.** PostgreSQL full-text search with the language-neutral `simple` configuration (no stemming, prefix matching for partial words; expected to suit English and Bangla, to be validated with real Bangla content in Phase 9) on a generated `tsvector` column, plus `pg_trgm` for names (people, spaces, channels, files, meetings). Every query is filtered by organisation and by the caller's readable conversations (the single read predicate in MULTITENANCY.md §3.4). The `search` module exposes an interface so a dedicated engine can replace it later.

## ADR-019 — Support context snapshot

**Decision.** When a support request includes conversation context, the API copies exactly the selected message (or the selected thread's root and replies) into `support_request_context_items` at creation time, after re-checking access. The copy is what is sent to the HelpDesk and what the request page shows. Nothing else from the conversation is ever transmitted. See [SUPPORT-AND-INTEGRATIONS.md](SUPPORT-AND-INTEGRATIONS.md).

## ADR-020 — Email notification content — *accepted*

**Decision.** Email notifications say *what happened and where* ("Ayesha mentioned you in #general") but exclude message text by default. Organisations may opt in to short previews. Rationale: email leaves the organisation's control boundary.

## ADR-021 — Roadmap adjustments

**Decision.** Keep the brief's phase order with these changes: platform mailer and background jobs move into Phases 1–2 (authentication needs email); security foundations (headers, CSRF, rate limiting, audit framework, CI scanning) start in Phases 1–2; search columns are created with their tables; DMs and group chats are explicit in Phase 4; Home is assembled incrementally. Details: [PHASES.md](../development/PHASES.md#changes-from-the-proposed-sequence).

## ADR-022 — V1 HelpDesk connector scope — *accepted*

**Context.** The brief requires a native HelpDesk/API connector *architecture* plus an SMTP fallback, with MateDesk "supported later through a native connector". MateDesk does not yet expose an API to integrate with.

**Decision.** V1 ships the connector framework (registry, capability flags, encrypted credentials, delivery jobs, webhooks/polling, delivery log), the SMTP connector, and a mock HelpDesk connector in tests that exercises the full native contract. The MateDesk connector is built as soon as its API is available.

**Consequences.** No native two-way HelpDesk sync at V1 launch; V1 includes SMTP delivery, connector capability handling, encrypted credentials, delivery jobs, the webhook/polling architecture, delivery logs and mock native-connector tests. The MateDesk connector follows its API.

## ADR-023 — Per-organisation presence, status and visible email

**Context.** Users can belong to several organisations (for example a member of their own company and a guest of a client). Global presence, status text or email lists would let one organisation observe a user's activity or addresses from another.

**Decision.** WebSocket connections are organisation-bound, so presence is computed per organisation; manual presence override and status text live on `organisation_memberships`; co-members see only the membership's `contact_email` (one of the user's verified addresses), never the `user_emails` list, which is visible to its owner only. Display name, avatar and time zone remain global identity fields.

**Consequences.** No cross-tenant leakage of activity or contact details; a user sets their status separately in each organisation (the UI can offer "apply to all my organisations").

## ADR-024 — Go version

**Context.** Go supports the two most recent major releases. Go 1.27 shipped in August 2026, ending security support for Go 1.25; the development machine currently has Go 1.25.14.

**Decision.** Target Go 1.27: `services/go.mod` declares `go 1.27.0` and `toolchain go1.27.2`; CI tests the pinned version; Renovate proposes upgrades when a new major release appears.

**Consequences.** *Implementation:* no machine-wide upgrade is needed. Any installed Go ≥ 1.21 reads the toolchain directive and downloads the checksum-verified Go 1.27.2 toolchain into the module cache (`GOTOOLCHAIN=auto`); the container build uses `golang:1.27-alpine` with `GOTOOLCHAIN=local`.

---

## ADR-025 — Guest policy — *accepted*

**Decision.** Guests are limited to explicitly granted Spaces (and the channels they may read there). Two organisation-controlled exceptions exist: guests may join meetings they are explicitly invited to when `meetings.guest_access` allows it, and guests may exchange direct messages only with users who share an accessible Space, controlled by `messaging.guest_direct_messages`, which **defaults to off**. Guests cannot create or be added to group chats. All guest access goes through the standard tenant isolation and authorization model.

---

## Open questions (need product or business input)

1. Organisation sign-up model for V1: invite-only with operator-created organisations, or self-service organisation creation? (Default: operator-created via `mcctl`, self-service behind a flag.)
2. Message edit history: retain revisions for compliance (default: yes, org-configurable, subject to retention)?
3. Breached-password checks: allow outbound k-anonymity calls to Have I Been Pwned, or use an offline list only?
4. Target production hosting (single VM, cloud provider, on-premises customer deployments?) — affects storage, TURN and backup design.
5. Production object storage provider (ADR-010).
6. Brand-owner requests in [BRANDING-ASSETS.md §7](../BRANDING-ASSETS.md#7-gaps--requests-for-the-brand-owner-no-engineering-workaround).
