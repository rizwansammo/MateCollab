# Security Baseline

Status: Phase 0 proposal · Applies to every phase · Related: [MULTITENANCY.md](../architecture/MULTITENANCY.md), [FILES.md](../architecture/FILES.md), [SUPPORT-AND-INTEGRATIONS.md](../architecture/SUPPORT-AND-INTEGRATIONS.md), [TESTING.md](../development/TESTING.md#security-tests)

"MUST" requirements are release-blocking for the phase that introduces the feature. Security work starts in Phase 1, not Phase 12; Phase 12 verifies and hardens.

---

## 1. Principles

1. **Secure by default** — the safe behaviour needs no configuration; unsafe options are explicit and audited.
2. **Least privilege** — for users (RBAC), services (database roles, storage keys, LiveKit grants) and code (module boundaries).
3. **Defence in depth** — no single check protects tenant data or credentials (see the isolation layers in [MULTITENANCY.md §2](../architecture/MULTITENANCY.md#2-isolation-layers)).
4. **Privacy by design** — collect the minimum, keep it the shortest time, share only what the user explicitly chose (support context, email previews).
5. **Verify, don't trust** — every request re-authenticates and re-authorises; client-supplied context is never trusted.

## 2. Threat model (summary)

| Asset | Main threats | Primary controls |
|---|---|---|
| Messages, files, meeting content | Cross-tenant access (IDOR, missing filters), over-privileged members, guest escalation | Org-in-path + membership middleware, authz policies, composite FKs, RLS, access-checked presigned URLs, cross-tenant tests |
| Accounts | Credential stuffing, phishing, session theft, CSRF | Argon2id, rate limiting, TOTP, HttpOnly `__Host-` cookies, CSRF tokens + Origin checks, session revocation |
| Integration secrets | Database leak, log leak, admin UI exposure | AES-256-GCM with AAD, write-only APIs, redaction, keyring rotation |
| Infrastructure | SSRF via admin-configured endpoints, exposed internal services | `netguard` dialer, private Docker networks, only Nginx published |
| Uploaded files | Malware distribution, stored XSS, decompression bombs | Scanning (fail closed), magic-byte typing, separate content origin, attachment disposition, bounded decoders |
| Meetings | Uninvited joins, token reuse, host impersonation | Server-issued short-lived single-room tokens, waiting room, API-mediated host controls |
| Audit trail | Tampering, deletion | Append-only grants, owner-only purge function, export |
| Availability | Abuse, floods, slow consumers | Multi-layer rate limits, body limits, WS backpressure, timeouts |

Actors considered: unauthenticated attacker; malicious member of another organisation; malicious member or guest of the same organisation; compromised Admin account; malicious or compromised integration endpoint; curious operator with database access.

Row-level security protects against **application** mistakes; it does not constrain anyone who can run arbitrary SQL (the tenant setting is a session variable). Operators are therefore controlled operationally: no standing access to production data, audited break-glass credentials, per-tenant `SECURITY DEFINER` views for support investigations, encrypted secrets that the database alone cannot decrypt, and encrypted backups.

## 3. Identity and authentication

| Control | Requirement |
|---|---|
| Password hashing | **MUST** use Argon2id via `golang.org/x/crypto/argon2`; initial parameters m = 64 MiB, t = 3, p = 1, 16-byte salt, 32-byte key (benchmarked to 150–300 ms; never below OWASP's 19 MiB/t=2 minimum); PHC-encoded; rehash on successful login when parameters change; a semaphore bounds concurrent hashes per process |
| Password policy | 12–128 characters, any Unicode (NFKC-normalised), no composition rules, no forced rotation; reject the top common/breached passwords from an offline list; optional Have I Been Pwned k-anonymity check (decision pending) |
| Login | Uniform responses and timing for unknown user / wrong password / disabled; progressive per-account delay after 5 failures (cap 30 s) plus per-IP limits; no permanent lockout |
| TOTP 2FA | RFC 6238 (SHA-1, 6 digits, 30 s — authenticator compatible); 160-bit secret encrypted at rest; enrolment requires a valid code before activation; ±1 step tolerance; replay prevention via `last_used_step` |
| Recovery codes | 10 single-use codes (~50 bits each, Crockford base32), stored as HMAC-SHA-256 keyed by a versioned keyring key whose ID is stored with each code (rotating keys never invalidates existing codes); shown once; regeneration invalidates old codes; warning when ≤ 2 remain |
| Organisation 2FA enforcement | `security.require_2fa` blocks organisation access (except enrolment) for members without TOTP |
| Step-up re-authentication | Password (and TOTP if enabled) within the last 10 minutes for: change password/email, disable 2FA, regenerate recovery codes, ownership transfer, integration secret changes, organisation deletion |
| Email verification and reset | Single-use 256-bit tokens, SHA-256 stored, 60 min (reset) / 24 h (verification); links carry the token in the URL **fragment** (never sent to servers, logs or `Referer`), the page POSTs it in the body, and token pages send `Referrer-Policy: no-referrer`; reset revokes all sessions; responses never reveal account existence |
| Invitations | Single-use tokens bound to the invited email; 7-day expiry; revocable; acceptance requires proving the email (token) and sets the role/grants recorded at invite time |
| Security notifications | Email on password change, 2FA change, recovery code use and new-device sign-in |
| Future SSO | OIDC/SAML can be added as additional authentication methods; sessions remain server-side |

## 4. Sessions

* Token: 32 random bytes (`crypto/rand`), base64url, in cookie `__Host-mc_session` — `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`, no `Domain`.
* Storage: SHA-256 of the token in `sessions`; cached in Redis (`mc:sess:{hash}`, 60 s TTL) and invalidated immediately on revocation.
* Lifetime: sliding idle timeout of 7 days and an absolute platform cap of 30 days. Sessions are global, so an organisation's `security.session_max_age` (absolute; default `30d` = the cap; options 12h/24h/7d/30d) is enforced **per request** to that organisation: if the session is older, that organisation's endpoints return `401 session_expired` and the user signs in again, while other organisations keep working.
* Rotation: new token on login and after password change; old tokens revoked.
* Revocation: per session, "sign out other sessions", automatic on password reset; revoked sessions close their WebSockets (`session.revoked`, 4009).
* Session list shows device (parsed user agent), approximate location only if a GeoIP database is configured (not in V1), last seen, and current-session marker.

## 5. CSRF and cross-origin

* All browser traffic is same-origin; **CORS is not enabled** for the API (preflights are rejected).
* Unsafe methods with a session require **both**: `Origin` in `MC_ALLOWED_ORIGINS` (or `Sec-Fetch-Site: same-origin` when `Origin` is absent) **and** `X-CSRF-Token` equal to `HMAC(csrf-subkey, session_id)` (constant-time compare).
* Unsafe methods **before a session exists** (login, MFA step, password forgot/reset, email verification, invitation acceptance, self-service sign-up) require an allowed `Origin` **and** the custom header `X-MC-Client: web`. Custom headers cannot be sent cross-site without a CORS preflight, which is never granted — this blocks login CSRF and cross-site form posts.
* Exempt: signature-verified webhooks and the rate-limited CSP report endpoint.
* `SameSite=Lax` adds a third layer. Logout and other state changes are never `GET`.
* WebSockets: cookie + single-use ticket (minted by a CSRF-protected `POST`) + `Origin` allow-list.

## 6. Authorization and tenant isolation

Covered normatively in [MULTITENANCY.md](../architecture/MULTITENANCY.md). Summary of MUSTs:

* Every tenant endpoint includes `{orgId}` and passes membership middleware.
* Every permission decision goes through `internal/authz`; handlers contain no role checks.
* Every tenant table has `org_id`, composite foreign keys, and forced RLS; the runtime database role cannot bypass RLS.
* Invisible resources return `404`; visible-but-forbidden return `403`.
* Every endpoint has an automated cross-tenant test and a role-matrix test.

## 7. Input validation and output encoding

* JSON decoding rejects unknown fields, duplicate keys and bodies over the route limit (`http.MaxBytesReader`).
* Every input type has explicit validation (lengths, enums, counts, formats, UUIDs). Text is normalised (NFC) and stripped of control and bidirectional-override characters in names and filenames.
* Messages are stored as markup text and rendered by a whitelist parser into React elements — no raw HTML, no `dangerouslySetInnerHTML`, URL schemes limited to `http`, `https`, `mailto`.
* Email headers built only through `go-mail` APIs (no manual header concatenation); CR/LF rejected in any header-bound value.
* SQL only through sqlc-generated parameterised queries; dynamic SQL (sorting) uses allow-listed identifiers.

## 8. Rate limiting and abuse prevention

* Redis GCRA limiter keyed by scope (IP, user, organisation, account, challenge) with per-route classes ([API-DESIGN.md §8](../api/API-DESIGN.md#8-rate-limiting)); Nginx `limit_req` as a coarse outer layer.
* Limits on cardinality: members per group (250), attachments per message (10), reactions per message (50 distinct), channels per Space (500), explicit WS subscriptions (50), presence subscriptions (500).
* Invitation and email-sending budgets per organisation prevent use of MateCollab as a spam relay.
* `@channel`/`@here` in large channels require confirmation and are rate-limited.

## 9. WebSocket security

See [REALTIME.md §13](../architecture/REALTIME.md#13-security): authenticated and org-bound connections, authorised subscriptions, per-recipient payload filtering, frame-size limits, command rate limits, immediate closure on revocation.

## 10. File upload security

See [FILES.md](../architecture/FILES.md): size enforced at three layers, block-listed extensions, magic-byte detection, ClamAV scanning that fails closed, bounded image decoding and re-encoding, downloads via 60 s presigned URLs with `attachment` disposition, never-inline SVG/HTML, separate content origin, quotas, retention.

## 11. HTTP security headers

Applied by Next.js `proxy.ts` (HTML) and the Go API (JSON); Nginx adds HSTS and strips version banners.

| Header | Value |
|---|---|
| `Content-Security-Policy` (HTML) | `default-src 'self'; script-src 'self' 'nonce-{nonce}' 'strict-dynamic'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: {storage}; media-src 'self' blob: {storage}; font-src 'self'; connect-src 'self' wss://{host} {livekit} {storage}; frame-src {storage}; worker-src 'self' blob:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests` |
| `Content-Security-Policy` (API) | `default-src 'none'; frame-ancestors 'none'` |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` (production only; `preload` after the domain is final) |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` (message links also use `rel="noopener noreferrer"`); `no-referrer` on pages that handle one-time tokens (reset, verification, invitation) |
| `Permissions-Policy` | `camera=(self), microphone=(self), display-capture=(self), fullscreen=(self), geolocation=(), payment=(), usb=(), bluetooth=()` |
| `Cross-Origin-Opener-Policy` | `same-origin` |
| `Cross-Origin-Resource-Policy` | `same-origin` |
| `X-Frame-Options` | `DENY` (legacy browsers) |
| `Cache-Control` (API, authenticated HTML) | `no-store` |

`style-src 'unsafe-inline'` is accepted because Radix positions popovers with inline `style` attributes; scripts remain nonce-restricted. CSP violation reports go to a rate-limited `/api/v1/csp-reports` endpoint (Phase 12).

## 12. Secrets and key management

* No secrets in the repository. `.env.example` contains placeholders only; `pnpm setup` generates local development secrets; gitleaks runs in CI and as a pre-commit hook.
* Runtime secrets come from environment variables or files (`*_FILE`, Docker secrets in production).
* Application-level encryption (`platform/crypto`): AES-256-GCM, random 96-bit nonces, keyring with key IDs (`MC_ENCRYPTION_KEYRING`), associated data binding ciphertext to its owner and purpose (e.g. `org_id|integration_id|integration-secret`, `user_id|totp`). Used for integration secrets and TOTP secrets.
* Key rotation: add a key, mark it active, run `mcctl keys rotate` to re-encrypt, remove the old key after verification. A KMS/Vault-backed key-encryption key can replace the environment keyring later without data-model changes.
* `MC_HMAC_SECRET` is expanded with HKDF into purpose-specific subkeys for **ephemeral** values only (CSRF tokens, cursor signing, WebSocket tickets), so one leaked derived key cannot forge others and rotation only forces clients to re-fetch those values. Anything persisted (recovery-code hashes, encrypted secrets) is keyed by the versioned keyring and records its key ID.
* Secrets are never logged, never returned by APIs, never stored in audit metadata, and only decrypted at the point of use.

## 13. Transport and network security

* TLS 1.2+ (prefer 1.3) at Nginx with modern ciphers; HTTP → HTTPS redirect; WebSockets only over `wss` in production.
* Docker networks: `edge` (Nginx), `app` (web, api, worker), `data` (PostgreSQL, Redis, storage, ClamAV). Only Nginx (and LiveKit media ports) are published in production. Local development binds published ports to `127.0.0.1`.
* Outbound TLS (SMTP, HelpDesk, storage) verifies certificates; plaintext SMTP only to allow-listed private relays.
* Redis requires authentication (ACL users per service); PostgreSQL requires password/SCRAM and, across hosts, TLS.

## 14. Outbound requests (SSRF)

All requests to administrator-configured destinations (SMTP hosts, HelpDesk URLs) use the `netguard` dialer: resolve once, block loopback/private/link-local/CGNAT/multicast/ULA/metadata addresses (unless allow-listed for on-premises), connect to the vetted IP, restrict ports, refuse redirects to blocked targets, cap response sizes and enforce timeouts. External link unfurling is out of V1 scope partly for this reason.

## 15. Audit logging

Append-only `audit_logs` (runtime role: `INSERT`/`SELECT` only). Each record: actor, action, target, outcome, IP, user agent, session ID, request ID, timestamp, and changed fields without secrets or message content.

| Category | Events |
|---|---|
| Authentication (account-level, stored in `account_audit_events`, not in any organisation's log) | login succeeded/failed (sampled under attack), logout, 2FA enabled/disabled, recovery codes regenerated/used, password changed/reset, email changed, session revoked |
| Organisation | profile/settings updated (section + changed keys), ownership transferred, deletion requested/cancelled |
| Membership | invitation created/revoked/accepted, role changed, suspended/reactivated/deactivated/removed, guest Space grants |
| Content governance | Space created/archived/deleted, channel visibility changed, message deleted by moderator, file deleted/quarantined |
| Meetings | ended for everyone, participant removed/muted by host |
| Integrations | created, updated, secret updated, tested, enabled/disabled, deleted |
| Support | request created (context mode, item and attachment counts), delivery failed |
| Audit | export requested/downloaded |
| Access control | denied attempts on administrative endpoints (sampled) |

Retention 1 year by default; Owners/Admins can filter and export (CSV) organisation events; users see their own account-level events in Settings → Security.

## 16. Logging and privacy

* Structured JSON logs with request ID, organisation ID, user ID, route and outcome. **Never** log message bodies, file contents, passwords, tokens, cookies, secrets, recovery codes, TOTP codes or full query strings on token-bearing routes.
* IP addresses and user agents are retained in logs for 30 days.
* Error responses never include stack traces or internal identifiers beyond `request_id`.
* Frontend error reporting (if enabled) scrubs URLs and user content.

## 17. Data protection and retention

* Encryption at rest: provider/volume encryption for PostgreSQL and backups; SSE for object storage in production.
* Retention policies per organisation (messages, files) and per platform (sessions, tokens, notifications, audit, logs) are enforced by scheduled jobs ([DATABASE.md §7](../architecture/DATABASE.md#7-deletion-retention-and-privacy)).
* Account deletion anonymises the user; organisation deletion purges all tenant data after a 30-day window.
* Notification emails exclude message content by default (ADR-020).
* Support requests transmit only the explicitly selected snapshot (ADR-019), and the stored snapshot follows the organisation's message retention.
* Data export (organisation and user) is planned post-V1.

## 18. Supply chain and secure development

* Lockfiles committed; Go `go.sum` verified; pnpm with `minimumReleaseAge` (7 days — kept from the prototype's policy), `strictDepBuilds` and an explicit build-script allow-list.
* Container base images pinned by digest; images built in CI, scanned (Trivy), SBOM generated (Syft); image signing (cosign) before GA.
* CI security gates: `govulncheck`, `pnpm audit` (high/critical fail), `gosec`, `staticcheck`/`golangci-lint`, ESLint security rules, Semgrep rules for authz/tenancy patterns, gitleaks, licence check (deny AGPL/SSPL in application dependencies without review).
* Dependency updates via Renovate with stability days; security updates fast-tracked.
* Pull requests touching `auth`, `authz`, `platform/crypto`, `platform/db`, migrations, `realtime`, `files`, `integrations` require a security-aware reviewer (CODEOWNERS) and the security checklist in the PR template.
* Lightweight threat modelling for each phase's features is recorded in the phase PR.
* DAST: OWASP ZAP baseline in CI against the Compose stack; full scan and an external penetration test before GA.
* Responsible disclosure: `security.txt` and a disclosure policy before GA.

## 19. Infrastructure hardening

* Containers run as non-root with read-only root filesystems, `cap_drop: [ALL]`, `no-new-privileges`, resource limits and health checks.
* Separate database roles (`mc_owner`, `mc_migrator`, `mc_app`); the application role cannot change schema or bypass RLS; operators have no standing access to production data (break-glass only).
* Object storage access key scoped to the MateCollab bucket only.
* Nginx: `server_tokens off`, request/header size limits, timeouts tuned for WebSockets, upload size limits only where the API accepts bodies (uploads go to storage).
* Backups encrypted, stored separately from production credentials, and restore-tested.

## 20. Monitoring and incident response

* Alerts: authentication failure spikes, rate-limit saturation, 5xx rate, job failures/exhaustion, scanner unavailability, signature age, integration delivery failures, webhook signature failures, unusual admin activity (bulk role changes, exports).
* Runbooks (Phase 12): revoke all sessions for a user/organisation, rotate encryption keys, rotate LiveKit/storage/SMTP credentials, disable an integration, quarantine a file, restore from backup.

## 21. Phase mapping

| Phase | Security deliverables |
|---|---|
| 1 | Security headers + CSP nonces, request IDs, structured redacted logging, body limits, CI security gates, secrets handling and `pnpm setup`, database roles + RLS helper functions, Docker network segmentation |
| 2 | Argon2id, sessions, CSRF, TOTP + recovery codes, step-up, rate limiting (auth classes), audit log framework, tenant/permission test harness |
| 3–5 | RLS + composite FKs on new tables, cross-tenant and role-matrix tests per endpoint, WebSocket authentication/authorization and limits |
| 6 | Upload pipeline security (scanning, typing, content origin) |
| 7–8 | LiveKit token grants, webhook verification, waiting room, host controls |
| 9 | Search access filtering, notification privacy, email content policy |
| 11 | Secret encryption, write-only APIs, `netguard`, connector webhook verification, support context guarantees |
| 12 | Threat model review, ZAP full scan, external penetration test, load/abuse tests, backup restore drill, incident runbooks, `security.txt` |

## 22. Open security decisions

1. Outbound breached-password checks (HIBP) vs offline list only.
2. Default retention of message edit history.
3. Email notification previews default (ADR-020 proposes off).
4. GeoIP-based session locations (privacy vs usefulness).
5. When to introduce SSO and SCIM for enterprise customers.
