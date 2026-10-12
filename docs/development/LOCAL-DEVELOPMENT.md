# Local Development

Status: Phase 0 plan · Implemented in Phase 1 · Related: [ADR-016](../architecture/DECISIONS.md#adr-016--task-runner), [OPERATIONS.md](../architecture/OPERATIONS.md)

Goal: a new developer clones the repository and runs the **entire stack with two commands**, on Windows, macOS or Linux.

```sh
pnpm setup     # once: checks tools, installs dependencies, generates .env with local secrets
pnpm dev       # every day: infrastructure in Docker + API, worker and web with hot reload
```

Then open **http://localhost:8080**.

> Nothing in this document configures real SMTP, real LiveKit infrastructure, production databases, domains or production secrets. All credentials below are generated, local-only values.

---

## 1. Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Docker Desktop (WSL 2 backend on Windows) | Compose v2+ | Give WSL at least 8 GB RAM (`%UserProfile%\.wslconfig`) if the ClamAV profile is used |
| Node.js | 22.13+ (24 LTS fine) | |
| pnpm | 11.x | `corepack enable` or standalone install |
| Go | 1.27.x | Go 1.25 is out of security support; this machine currently has 1.25.14 and needs upgrading |
| Git | 2.4x+ | `core.longpaths=true` on Windows |

Not required: `make`, a local PostgreSQL client (use `pnpm db:psql`), Python.

## 2. Two ways to run

| Mode | Command | Best for |
|---|---|---|
| **Hybrid** (default) | `pnpm dev` | Daily development: infrastructure + Nginx in Docker; API, worker (`air` hot reload) and web (`next dev`) run natively for fast reloads |
| **Full Docker** | `pnpm dev:docker` | Reviewers, QA, CI parity: everything in containers (`docker-compose.yml` + `docker-compose.dev.yml`) |

In both modes Nginx serves the single origin `http://localhost:8080` (web + `/api` + `/api/v1/realtime`), exactly like production. In hybrid mode Nginx proxies to `host.docker.internal:3000` (web) and `:8081` (API).

## 3. Services

All published ports bind to `127.0.0.1`. Image versions are pinned by digest in Phase 1.

| Service | Image | Host port(s) | Purpose | Profile |
|---|---|---|---|---|
| `nginx` | `nginx` (alpine) | 8080 | Single origin, WebSocket upgrade | core |
| `postgres` | `postgres:18` | 5432 | Database; the init script is the local **bootstrap** (needs superuser): creates roles `mc_owner`, `mc_migrator` (BYPASSRLS), `mc_app`, database `matecollab`, extension `pg_trgm`. Tables, functions, grants and RLS come from migrations | core |
| `redis` | `valkey/valkey:8` (or `redis:8`, ADR-011) | 6379 | Pub/sub, presence, cache, rate limits (password-protected) | core |
| `minio` | last published MinIO community image, pinned (ADR-010) | 9000 (S3), 9001 (console) | Object storage | core |
| `livekit` | `livekit/livekit-server` | 7880 (HTTP/WS), 7881/tcp, 7882/udp | SFU with development keys | core |
| `mailpit` | `axllent/mailpit` | 8025 (UI + API), 1025 (SMTP) | Captures every email; nothing leaves the machine | core |
| `migrate` | built from `infra/docker/mcctl.Dockerfile` | — | One-shot `mcctl migrate up` | core |
| `api`, `worker`, `web` | built from `infra/docker/*.Dockerfile` | internal | Application (full Docker mode only) | app |
| `clamav` | `clamav/clamav` | 3310 | Real malware scanning (`MC_SCANNER=clamav`) | scan |
| `otel-lgtm` | `grafana/otel-lgtm` | 3001 (Grafana), 4317/4318 (OTLP) | Logs, metrics, traces | observability |

Enable optional profiles with `pnpm dev --scan` / `--observability` (wrappers around `docker compose --profile …`).

Illustrative excerpt of the planned `docker-compose.dev.yml` (the real file is created in Phase 1):

```yaml
services:
  postgres:
    image: postgres:18            # pinned by digest in Phase 1
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/pg_superuser
    volumes:
      - pgdata:/var/lib/postgresql
      - ./infra/docker/postgres/init:/docker-entrypoint-initdb.d:ro
    ports: ["127.0.0.1:5432:5432"]
    healthcheck: { test: ["CMD", "pg_isready", "-U", "postgres"], interval: 5s }
  livekit:
    image: livekit/livekit-server  # pinned in Phase 1
    command: ["--config", "/etc/livekit.yaml"]
    volumes: ["./infra/livekit/livekit.dev.yaml:/etc/livekit.yaml:ro"]
    ports: ["127.0.0.1:7880:7880", "127.0.0.1:7881:7881", "127.0.0.1:7882:7882/udp"]
  mailpit:
    image: axllent/mailpit         # pinned in Phase 1
    ports: ["127.0.0.1:8025:8025", "127.0.0.1:1025:1025"]
volumes:
  pgdata: {}
```

## 4. Environment and secrets

* `.env.example` (committed) lists every variable with safe placeholders ([BACKEND.md §8](../architecture/BACKEND.md#8-configuration)).
* `pnpm setup` copies it to `.env` (git-ignored) and fills random values for database passwords, Redis password, MinIO keys, LiveKit dev key/secret, `MC_HMAC_SECRET` and the encryption keyring.
* Local defaults: `MC_ENV=local`, `MC_PUBLIC_URL=http://localhost:8080`, `MC_SCANNER=noop`, `MC_SMTP_HOST=localhost` (hybrid) or `mailpit` (Docker), `MC_SMTP_TLS=none` (allowed only when `MC_ENV=local`).
* Browsers treat `http://localhost` as a secure context, so `__Host-`/`Secure` cookies, camera and microphone work without TLS. Testing from another device on the LAN needs HTTPS (optional `mkcert` instructions in Phase 1).

## 5. Database

| Task | Command |
|---|---|
| Apply migrations | `pnpm db:migrate` (runs `mcctl migrate up` with `mc_migrator`) |
| Roll back one | `pnpm db:rollback` |
| Status | `pnpm db:status` |
| Seed demo data | `pnpm db:seed` (refuses to run unless `MC_ENV=local`) |
| Reset (drop volume, migrate, seed) | `pnpm db:reset` (asks for confirmation) |
| SQL shell | `pnpm db:psql` (`docker compose exec postgres psql -U mc_app matecollab`) |
| Regenerate sqlc code | `pnpm gen` |

Seed data creates a demo organisation, Spaces, channels, DMs, meetings, files metadata and support requests resembling the prototype, using **fictional people only**. Your own Owner account: `pnpm mcctl user create-owner --org demo --email you@example.com` prints a one-time password-setup link (the email also lands in Mailpit).

## 6. Object storage (MinIO)

* `pnpm setup` / `pnpm infra:up` runs `mcctl storage init` (bucket `matecollab`, lifecycle rules).
* Browser uploads go to `http://localhost:9000` via presigned requests; the API reaches MinIO at `http://minio:9000` (Docker) and signs with the public endpoint (`MC_S3_PUBLIC_ENDPOINT`).
* MinIO's server-wide CORS is set to `http://localhost:8080` in Compose.
* The MinIO console at `http://localhost:9001` is for inspection only.
* MinIO upstream is archived (ADR-010); it is used here strictly as a local S3 stand-in.

## 7. LiveKit

* Runs `livekit-server` with `infra/livekit/livekit.dev.yaml`:

```yaml
port: 7880
rtc:
  tcp_port: 7881
  udp_port: 7882          # single UDP port for local development
  use_external_ip: false
  node_ip: 127.0.0.1      # advertise localhost to the browser on Docker Desktop
room:
  auto_create: false      # rooms are created by the API only
keys:
  devkey: <generated by pnpm setup>
webhook:
  api_key: devkey
  urls:
    - http://host.docker.internal:8081/api/v1/webhooks/livekit   # hybrid mode (Docker mode: http://api:8081/…)
```

* The browser connects to `ws://localhost:7880`; media flows over `127.0.0.1:7882/udp` (TCP 7881 fallback).
* To test with two people, use two browser profiles (or one normal + one private window) with different accounts. Chromium's `--use-fake-device-for-media-stream` gives synthetic camera/microphone for automated tests.
* No TURN, domain or certificates are configured locally.

## 8. Email (Mailpit)

* Every email (invitations, verification, reset, notifications, SMTP-fallback support tickets, integration test emails) is captured at **http://localhost:8025**.
* Organisation SMTP integrations created in the local admin UI should point to `mailpit:1025` (Docker) or `localhost:1025` (hybrid); `netguard` allows these private addresses only when `MC_ENV=local`.
* Playwright tests assert email contents through Mailpit's HTTP API.

## 9. Malware scanning (optional)

* Default `MC_SCANNER=noop`: files become available after type/size checks and display "Not scanned (development)".
* `pnpm dev --scan` starts ClamAV (first start downloads signatures, ~1–2 minutes, ~1.5–3 GB RAM) and sets `MC_SCANNER=clamav`.
* Verify with the EICAR test file: upload must end in `quarantined`.

## 10. Local URLs

| URL | What |
|---|---|
| http://localhost:8080 | MateCollab (web + API through Nginx) |
| http://localhost:8080/api/v1/config | Runtime config (smoke check) |
| http://localhost:8025 | Mailpit |
| http://localhost:9001 | MinIO console |
| http://localhost:6006 | Storybook (`pnpm storybook`) |
| http://localhost:3001 | Grafana (observability profile) |
| http://localhost:8091/metrics · /healthz · /readyz | API admin endpoints (hybrid mode) |
| http://localhost:8092/metrics · /healthz · /readyz | Worker admin endpoints (hybrid mode) |

## 11. Everyday commands (planned root `package.json` scripts)

| Command | Purpose |
|---|---|
| `pnpm setup` | Check prerequisites, install JS and Go dependencies, generate `.env`, sync brand assets |
| `pnpm dev` / `pnpm dev:docker` | Hybrid / full-Docker stack |
| `pnpm infra:up` · `infra:down` · `infra:reset` | Infrastructure containers only; reset removes volumes (confirmation) |
| `pnpm db:*` | See §5 |
| `pnpm gen` | sqlc, OpenAPI → TypeScript, event schemas → TypeScript |
| `pnpm lint` · `pnpm typecheck` | golangci-lint + ESLint + TypeScript |
| `pnpm test` | Go unit + integration, web unit (see [TESTING.md](TESTING.md)) |
| `pnpm test:e2e` | Playwright against the full stack |
| `pnpm test:security` | Security test suite (ZAP baseline, header checks, authz fuzzing) |
| `pnpm storybook` | Design system and state catalogue |
| `pnpm brand:sync` | Copy approved assets from `MateCollab-Logo-Library` with hash verification |
| `pnpm mcctl …` | Run the admin CLI against local services |

## 12. Hot reload and debugging

* Go: `air` rebuilds `api`/`worker` on change; Delve via VS Code launch configurations (`.vscode/launch.json` in Phase 1).
* Web: `next dev` with Fast Refresh; React DevTools; TanStack Query Devtools in development builds only.
* Logs: `MC_LOG_FORMAT=text` locally for readability; request IDs shown in UI error toasts to correlate.

## 13. Windows notes

* Commit `.gitattributes` with `* text=auto eol=lf` so shell scripts and SQL keep LF endings; set `git config core.autocrlf false`.
* Prefer **hybrid mode**: bind-mounted source inside Linux containers is slow to watch on Windows. For full Docker mode, keep the repository in the WSL 2 filesystem or enable polling (`WATCHPACK_POLLING=true`, `air` poll mode).
* If port 5432 is taken by another PostgreSQL, change `MC_PG_HOST_PORT` in `.env`.
* `host.docker.internal` is available on Docker Desktop; on Linux Compose adds it via `extra_hosts: ["host.docker.internal:host-gateway"]`.

## 14. Troubleshooting

| Symptom | Check |
|---|---|
| Login works but realtime does not | Browser console for WS close code; `MC_ALLOWED_ORIGINS` must include `http://localhost:8080`; Nginx upgrade headers |
| Uploads fail with CORS errors | MinIO CORS origin; `MC_S3_PUBLIC_ENDPOINT` must be `http://localhost:9000` |
| Presigned URL "signature does not match" | Presigning must use the public endpoint client |
| Calls connect but no media | UDP 7882 published; `node_ip: 127.0.0.1`; firewall prompts for Docker Desktop |
| No emails | Mailpit running; `MC_SMTP_HOST`/`PORT` for the current mode |
| API refuses to start | Pending migrations (`pnpm db:migrate`) or invalid `.env` (`pnpm mcctl config check`) |
