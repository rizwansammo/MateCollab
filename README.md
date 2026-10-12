# MateCollab

Enterprise collaboration platform by **NetaMate Solutions**: chats, Spaces, meetings, files and
activity for multi-tenant organisations.

## Quick start

Requirements: Docker Desktop (or Docker Engine), Node.js 24+, pnpm 11, and any Go ≥ 1.21 on
`PATH` (the Go 1.27 toolchain is fetched automatically from `services/go.mod`).

```sh
pnpm setup   # once: .env with local secrets, dependencies, infrastructure, database, storage
pnpm dev     # infrastructure in Docker; API, worker and web natively with reload
```

Open **http://localhost:8080**. Everything (web, `/api`, WebSocket) is served from that one
origin through Nginx, exactly as in production. `pnpm dev:docker` runs the whole stack in
containers instead. Details: [docs/development/LOCAL-DEVELOPMENT.md](docs/development/LOCAL-DEVELOPMENT.md).

## Repository layout

| Path | Contents |
|---|---|
| `apps/web` | Next.js web application (App Router, React 19, Tailwind 4) |
| `packages/ui` | Design system: tokens, accessible primitives, Storybook |
| `packages/contracts` | OpenAPI 3.1 + WebSocket event schemas and the TypeScript types generated from them |
| `packages/config` | Shared TypeScript and ESLint configuration |
| `services` | Go module: `api`, `worker`, `mcctl` binaries and the shared `internal` packages |
| `db/migrations` | goose SQL migrations (`db/queries`, `db/seeds` follow with product data) |
| `infra` | Dockerfiles, Nginx, LiveKit and monitoring configuration |
| `scripts` | Cross-platform Node scripts behind the root `pnpm` commands |
| `docs` | Architecture, API, security and development documentation |
| `MateCollab-Logo-Library` | Official brand source (read-only; synced with `pnpm brand:sync`) |

## Common commands

| Command | Purpose |
|---|---|
| `pnpm check` | Formatting, lint, types, unit tests, contracts, brand and licence checks |
| `pnpm test:go:integration` | Go integration tests against real PostgreSQL, Valkey, MinIO and Mailpit |
| `pnpm storybook` | Design system catalogue at http://localhost:6006 |
| `pnpm gen` | Regenerate contract types after editing `packages/contracts` |
| `pnpm doctor` | Check every local dependency |

## Documentation

Start with [docs/README.md](docs/README.md). Architecture decisions are recorded in
[docs/architecture/DECISIONS.md](docs/architecture/DECISIONS.md).

## Licence

Proprietary. © NetaMate Solutions. All rights reserved. The MateCollab name and logo are
trademarks of NetaMate Solutions; brand assets are used only as provided by the Logo Library.
