# MateCollab Documentation

MateCollab is NetaMate Solutions' enterprise collaboration platform: messaging, Spaces and channels, calls, huddles and meetings, files, search, notifications, action items and support requests from conversations — simpler than Slack, Teams or Discord by design.

**Current status:** Phase 0 (architecture, audit and planning) is approved. Production foundation work is under way. See [development/PHASES.md](development/PHASES.md).

## Sources of truth

| Topic | Authority |
|---|---|
| Logos, brand colours, wordmark | `MateCollab-Logo-Library/` — see [BRANDING-ASSETS.md](BRANDING-ASSETS.md) |
| Visual language and UX flows | MateCollab-UI prototype (`github.com/rizwansammo/MateCollab-UI`) — reference only; audited in [architecture/PHASE-0-AUDIT.md](architecture/PHASE-0-AUDIT.md) |
| Architecture, contracts, plans | This `docs/` folder |
| Decisions and deviations | [architecture/DECISIONS.md](architecture/DECISIONS.md) |

## Suggested reading order

1. [architecture/OVERVIEW.md](architecture/OVERVIEW.md) — system context, modules, key flows, technology choices
2. [architecture/DECISIONS.md](architecture/DECISIONS.md) — what was decided and why
3. [development/PHASES.md](development/PHASES.md) — roadmap and exit criteria
4. Topic documents as needed (below)

## Index

### Architecture

| Document | Contents |
|---|---|
| [OVERVIEW.md](architecture/OVERVIEW.md) | Scope, principles, containers, module map, key flows, consistency model, technology choices, NFR targets |
| [DECISIONS.md](architecture/DECISIONS.md) | Architecture decision record (ADR-001 … ADR-025) and open questions |
| [PHASE-0-AUDIT.md](architecture/PHASE-0-AUDIT.md) | Workspace audit, prototype audit (keep / adapt / discard), defects to correct |
| [FRONTEND.md](architecture/FRONTEND.md) | Next.js architecture, routes, shell layout, design tokens, theming, markup, accessibility, performance |
| [BACKEND.md](architecture/BACKEND.md) | Go modular monolith, module catalogue, layering, jobs, notifications, search, email, configuration |
| [REALTIME.md](architecture/REALTIME.md) | WebSocket gateway, Redis fanout, ordering and resync, read state, typing, presence |
| [DATABASE.md](architecture/DATABASE.md) | Conventions, normalisation decisions, ERDs, table specifications, indexes, retention, migrations |
| [MULTITENANCY.md](architecture/MULTITENANCY.md) | Tenant model, nine isolation layers, row-level security, roles and permission catalogue, guest rules |
| [MEDIA.md](architecture/MEDIA.md) | LiveKit integration: meeting kinds, join flow, grants, rooms, webhooks, huddles, calls |
| [FILES.md](architecture/FILES.md) | Object storage, presigned uploads, scanning, previews, access policy, retention |
| [SUPPORT-AND-INTEGRATIONS.md](architecture/SUPPORT-AND-INTEGRATIONS.md) | Support requests, explicit context sharing, connector framework, SMTP fallback, organisation SMTP |
| [OPERATIONS.md](architecture/OPERATIONS.md) | Topology, release process, observability, backups and DR, maintenance jobs |

### API

| Document | Contents |
|---|---|
| [API-DESIGN.md](api/API-DESIGN.md) | Versioning, authentication, conventions, pagination, errors, idempotency, rate limits, resource catalogue |
| [WEBSOCKET-EVENTS.md](api/WEBSOCKET-EVENTS.md) | Handshake, envelopes, versioning, close codes, commands, event catalogue, examples |

### Security

| Document | Contents |
|---|---|
| [SECURITY-BASELINE.md](security/SECURITY-BASELINE.md) | Threat model, authentication, sessions, CSRF, headers, secrets, SSRF, audit, logging, supply chain, phase mapping |

### Development

| Document | Contents |
|---|---|
| [LOCAL-DEVELOPMENT.md](development/LOCAL-DEVELOPMENT.md) | Two-command local stack, services and ports, LiveKit/MinIO/Mailpit/ClamAV, Windows notes |
| [TESTING.md](development/TESTING.md) | Test matrix, tenant isolation, permissions, realtime, LiveKit, frontend, E2E, security, CI |
| [PHASES.md](development/PHASES.md) | Phase 0–12 roadmap with scope and exit criteria |

### Brand

| Document | Contents |
|---|---|
| [BRANDING-ASSETS.md](BRANDING-ASSETS.md) | Logo Library inventory, usage map, prototype conflicts, gaps, HemiHead licensing note |

## Conventions

* British English for product terms (organisation, behaviour, colour).
* Diagrams are Mermaid (render on GitHub and in most Markdown viewers).
* "MUST" statements in the security baseline are release-blocking.
* Each phase updates the documents it affects; decisions are appended to `DECISIONS.md`, never silently rewritten.
