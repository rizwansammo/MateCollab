# Local Development

Status: Implemented (production foundation) · Related: [ADR-016](../architecture/DECISIONS.md#adr-016--task-runner), [OPERATIONS.md](../architecture/OPERATIONS.md)

Goal: a new developer clones the repository and runs the **entire stack with two commands**, on Windows, macOS or Linux.

```sh
pnpm setup     # once (safe to repeat): checks tools, generates .env, installs, prepares database and storage
pnpm dev       # every day: infrastructure in Docker + API, worker and web natively with reload
```

Then open **http://localhost:8080**.

> Nothing in this document configures real SMTP, real LiveKit infrastructure, production databases, domains or production secrets. All credentials are generated, local-only values.

---

## 1. Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Docker Desktop (WSL 2 backend on Windows) or Docker Engine | Compose v2.24+ | Give Docker at least 8 GB RAM if the ClamAV profile is used |
| Node.js | 24 LTS or newer | |
| pnpm | 11.x | `corepack enable` or standalone install. pnpm 11 lets the root `setup` script override pnpm's built-in `setup` command |
| Go | any Go ≥ 1.21 on `PATH` | `services/go.mod` declares `toolchain go1.27.2`; the `go` command downloads and checksum-verifies Go 1.27.2 automatically (`GOTOOLCHAIN=auto`, the default). No system-wide upgrade or admin rights needed |
| Git | 2.4x+ | `core.longpaths=true` on Windows |

Not required: `make`, `air`, a local PostgreSQL client (use `pnpm db:psql`), Python.

## 2. Two ways to run

| Mode | Command | Best for |
|---|---|---|
| **Hybrid** (default) | `pnpm dev` | Daily development: infrastructure + Nginx in Docker; API, worker and web (`next dev`) run natively. Go sources are watched: on change the binaries are rebuilt and restarted (a failed build keeps the previous processes running); a changed migration is applied first |
| **Full Docker** | `pnpm dev:docker` | Reviewers, QA, CI parity: everything in containers built from the production Dockerfiles (`--logs` follows logs) |

In both modes Nginx serves the single origin `http://localhost:8080` (web + `/api` + `/api/v1/realtime`), exactly like production. In hybrid mode Nginx proxies to `host.docker.internal:3000` (web) and `:8081` (API); the native processes listen on `127.0.0.1` only. `pnpm dev` removes containers left over from `pnpm dev:docker` so two workers never share a queue.

Optional profiles: `pnpm dev --scan` (ClamAV, sets `MC_SCANNER=clamav`) and `pnpm dev --observability` (Grafana LGTM, exports traces to it). `pnpm infra:up --scan --observability` starts them without the app.

## 3. Services

Defined in `docker-compose.yml` (+ `docker-compose.dev.yml` for local port publishing). All published ports bind to `127.0.0.1`; every image is pinned by digest.

| Service | Image | Host port(s) | Purpose | Profile |
|---|---|---|---|---|
| `nginx` | `nginx:1.30.5-alpine` | 8080 | Single origin, WebSocket upgrade, request IDs, query-string-free access log | core |
| `postgres` | `postgres:18.6` | **15432** | Database. Roles and database are created by `mcctl db bootstrap` (§5), not by an init script | core |
| `valkey` | `valkey/valkey:8.1.10-alpine` (ADR-011) | 6379 | Pub/sub, presence, cache, rate limits (password-protected, no persistence) | core |
| `minio` | `matecollab/minio-local`, **built locally** from the final community source release (ADR-010) | 9000 (S3), 9001 (console) | Object storage stand-in | core |
| `livekit` | `livekit/livekit-server:v1.13.9` | 7880 (HTTP/WS), 7881/tcp, 7882/udp | SFU with generated development keys | core |
| `mailpit` | `axllent/mailpit:v1.31.4` | 8025 (UI + API), 1025 (SMTP) | Captures every email; nothing leaves the machine | core |
| `migrate` | `infra/docker/go.Dockerfile` target `mcctl` | — | One-shot `mcctl prepare` before the API and worker start | app |
| `api`, `worker`, `web` | `infra/docker/go.Dockerfile` (targets `api`, `worker`), `infra/docker/web.Dockerfile` | internal | Application (full Docker mode only) | app |
| `clamav` | `clamav/clamav:1.5.4` | 3310 | Real malware scanning | scan |
| `otel-lgtm` | `grafana/otel-lgtm:0.35.0` | 3001 (Grafana), 4317/4318 (OTLP) | Logs, metrics, traces | observability |

**Why MinIO is built locally:** MinIO no longer publishes community images and the upstream repository is archived. `infra/docker/minio.Dockerfile` clones tag `RELEASE.2025-10-15T17-29-55Z`, verifies it resolves to commit `9e49d5e7…`, and builds it (first `pnpm setup` takes a few extra minutes). It is never used outside local development; production storage is chosen separately (ADR-010).

## 4. Environment and secrets

* `.env.example` (committed) lists every variable; `__TOKEN__` placeholders mark secrets.
* `pnpm setup` copies it to `.env` (git-ignored) and replaces each token with a fresh random value: database passwords (superuser, `mc_migrator`, `mc_app`), Valkey password, MinIO root password, LiveKit API secret, `MC_HMAC_SECRET` and a 32-byte encryption keyring entry.
* Re-running `pnpm setup` **keeps existing values** and only appends variables added to the template since.
* Local defaults: `MC_ENV=local`, `MC_PUBLIC_URL=http://localhost:8080`, `MC_SCANNER=noop`, `MC_SMTP_TLS=none` (accepted only when `MC_ENV=local`), `MC_LOG_FORMAT=text`.
* Containers receive the same `.env` plus Docker-network hostnames from `docker-compose.yml` (`postgres:5432`, `valkey:6379`, `minio:9000`, `mailpit:1025`, `livekit:7880`).
* Browsers treat `http://localhost` as a secure context, so `__Host-`/`Secure` cookies, camera and microphone work without TLS. Testing from another device on the LAN needs HTTPS (mkcert; documented when needed).

## 5. Database

Three roles (see [DATABASE.md](../architecture/DATABASE.md) and [MULTITENANCY.md](../architecture/MULTITENANCY.md)):

| Role | Login | Purpose |
|---|---|---|
| `mc_owner` | no | Owns every object; `BYPASSRLS`. Nobody logs in as it |
| `mc_migrator` | yes | Member of `mc_owner`; migrations run as `SET ROLE mc_owner` |
| `mc_app` | yes | API and worker. `NOBYPASSRLS`, no DDL; tenant scope comes from transaction-local `mc.org_id` / `mc.user_id` |

`mcctl prepare` = `db bootstrap` (when `MC_DATABASE_BOOTSTRAP_URL` is set) → `migrate up` (application schema via goose, then River's job tables) → `storage init`. It is idempotent and runs on every `pnpm setup`, every `pnpm dev` start and in the `migrate` container. The API and worker refuse to start when the schema is behind.

| Task | Command |
|---|---|
| Apply migrations | `pnpm db:migrate` |
| Roll back the latest | `pnpm db:rollback` |
| Status | `pnpm db:status` |
| Re-run role bootstrap | `pnpm db:bootstrap` |
| SQL shell (superuser, `matecollab` database) | `pnpm db:psql` |
| Delete all local data | `pnpm infra:reset --yes` (then `pnpm dev` recreates everything) |

Migrations live in `db/migrations` (`YYYYMMDDHHMMSS_name.sql`, goose `-- +goose Up/Down`). Queries live in `db/queries/<module>/*.sql`; `pnpm gen` (or `pnpm gen:sql`) regenerates the typed Go code in `services/internal/<module>/store` with sqlc from its pinned image, and `pnpm sql:check` fails when that code is stale.

### First organisation and sign-in

Self-service sign-up is off (`MC_ALLOW_ORG_SIGNUP=false`), so organisations are created by an operator:

```sh
pnpm mcctl org create --name "Acme Corporation" --slug acme --owner-email you@example.test --owner-name "Your Name"
```

* A new owner account receives a **set-your-password** email (open it in Mailpit, http://localhost:8025); the link works once and expires in 72 hours. An existing account simply becomes the Owner. `--print-setup-link` also prints the link (treat it like a password).
* Sign in at http://localhost:8080 → you land on `/{slug}/home`. Invite people from **Administration → Invitations**; their invitation emails also arrive in Mailpit.
* To give an existing organisation a new Owner (bootstrap after an import, or recovery when the Owner can no longer sign in): `pnpm mcctl user create-owner --org acme --email new-owner@example.test [--name "…"] [--print-setup-link]`. A new account gets the same one-time set-your-password email (no password is ever generated); an existing account is promoted; the previous Owner stays as an Admin and the change is audited. When the Owner can sign in, use **Administration → General → Transfer ownership** instead.
* In Docker mode without Go installed: `docker compose -f docker-compose.yml -f docker-compose.dev.yml --profile app run --rm --no-deps migrate org create …`.
* Sign-in, recovery and invitation attempts are rate limited per client address. Locally every browser request arrives from the Docker gateway address, so heavy manual testing can hit the limits; `pnpm test:e2e` resets them before it runs.

## 6. Object storage (MinIO)

* `mcctl storage init` creates bucket `matecollab` and applies lifecycle rules: objects under `tmp/` expire after one day. MinIO rejects the abort-incomplete-multipart rule (it expires stale multipart uploads itself), so it is reported as skipped; S3-compatible production providers receive both rules.
* MinIO does not support per-bucket CORS; Compose sets server-wide CORS to `MC_PUBLIC_URL` (`MINIO_API_CORS_ALLOW_ORIGIN`).
* Browser uploads and downloads use `http://localhost:9000` through presigned URLs signed with the public endpoint (`MC_S3_PUBLIC_ENDPOINT`); presigned downloads always force `Content-Disposition: attachment`.
* The MinIO console at `http://localhost:9001` is for inspection only (user `mc-minio-admin`, password in `.env`).

## 7. LiveKit

* `livekit-server` runs with `infra/livekit/livekit.dev.yaml` (port 7880, TCP 7881, single UDP port 7882, `node_ip: 127.0.0.1`, `auto_create: false`). The API key/secret come from `.env` through `LIVEKIT_KEYS`.
* The browser will connect to `ws://localhost:7880` (`media.livekit_url` in `/api/v1/config`). The webhook target is added together with meetings.
* No TURN, domain or certificates are configured locally.

## 8. Email (Mailpit)

* Every email is captured at **http://localhost:8025**: account setup, password reset, email verification, invitations and security notifications. The worker delivers them from the `email` queue, so they need the worker running. Send a test message: `pnpm mcctl mail test --to you@example.com`.
* Organisation SMTP integrations created later in the local admin UI point to `mailpit:1025` (Docker) or `localhost:1025` (hybrid).

## 9. Malware scanning (optional)

* Default `MC_SCANNER=noop`: files will display "Not scanned (development)".
* `pnpm dev --scan` starts ClamAV (first start downloads signatures, 1–2 minutes and ~1.5–3 GB RAM) and sets `MC_SCANNER=clamav`.

## 10. Local URLs

| URL | What |
|---|---|
| http://localhost:8080 | MateCollab (web + API through Nginx) |
| http://localhost:8080/api/v1/config | Runtime config (smoke check) |
| http://localhost:8025 | Mailpit |
| http://localhost:9001 | MinIO console |
| http://localhost:6006 | Storybook (`pnpm storybook`) |
| http://localhost:3001 | Grafana (observability profile) |
| http://127.0.0.1:8091/healthz · /readyz · /metrics | API admin endpoints (hybrid mode) |
| http://127.0.0.1:8092/healthz · /readyz · /metrics | Worker admin endpoints (hybrid mode) |

## 11. Everyday commands

| Command | Purpose |
|---|---|
| `pnpm setup` | Prerequisites, `.env`, dependencies, brand sync, contract generation, infrastructure, Go build, `mcctl prepare`, `mcctl doctor` (`pnpm run setup -- --skip-install` skips `pnpm install`) |
| `pnpm dev` / `pnpm dev:docker` | Hybrid / full-Docker stack |
| `pnpm infra:up` · `infra:down` · `infra:reset --yes` · `infra:logs [svc]` · `infra:ps` | Infrastructure containers |
| `pnpm db:*` | See §5 |
| `pnpm doctor` | Connectivity and schema check of every dependency |
| `pnpm mcctl …` | Operator CLI against local services (`org create`, `config check`, `keys generate`, `mail test`, …) |
| `pnpm gen` | OpenAPI and event schemas → bundled JSON + TypeScript types; SQL queries → sqlc Go code (`gen:contracts` / `gen:sql` run one side) |
| `pnpm format` · `format:check` | Prettier + gofmt |
| `pnpm lint` · `pnpm typecheck` | ESLint, Redocly, `go vet` · TypeScript |
| `pnpm test` | Web/contract unit tests + Go unit tests |
| `pnpm test:go:integration` | Go integration tests (throwaway PostgreSQL container + local Valkey/MinIO/Mailpit) |
| `pnpm test:e2e` | Playwright end-to-end flows and screenshots against the running stack |
| `pnpm sql:check` | sqlc code is current |
| `pnpm contracts:check` | Generated contract files fresh, OpenAPI lint, schema tests |
| `pnpm storybook` · `storybook:build` | Design system and state catalogue |
| `pnpm visual:test` · `visual:update` | Storybook visual baselines (light and dark) with axe checks |
| `pnpm brand:sync` · `brand:check` | Copy approved assets from `MateCollab-Logo-Library` with hash verification |
| `pnpm licenses` · `pnpm audit:deps` | Licence allow-list · npm audit + govulncheck |
| `pnpm check` | Everything CI runs that does not need Docker |

## 12. Reload and debugging

* Go: `pnpm dev` rebuilds `services/bin/{api,worker}` on `.go`/`go.mod` changes and restarts both; attach Delve to the running process if needed.
* Web: `next dev` with Fast Refresh.
* Logs: `MC_LOG_FORMAT=text` locally; every line carries `request_id`, which is also returned in the `X-Request-ID` header and in problem responses.

## 13. Windows notes

* `.gitattributes` keeps LF endings (`* text=auto eol=lf`); set `git config core.autocrlf false`.
* PostgreSQL is published on **15432** so it never collides with a natively installed PostgreSQL on 5432. To change it, edit `MC_PG_HOST_PORT` and the port in the three database URLs in `.env`.
* Prefer hybrid mode: watching bind-mounted source inside Linux containers is slow on Windows.
* `host.docker.internal` is provided by Docker Desktop; on Linux Compose adds it via `extra_hosts: host-gateway`.

## 14. Troubleshooting

| Symptom | Check |
|---|---|
| `ports are not available` on `pnpm setup`/`pnpm dev` | Another process owns the port (`netstat -ano`); change the host port in `.env`/`docker-compose.dev.yml` |
| 502 from http://localhost:8080 | The native web or API process is not running (`pnpm dev` output) |
| Uploads fail with CORS errors | `MC_PUBLIC_URL` (MinIO CORS origin) and `MC_S3_PUBLIC_ENDPOINT=http://localhost:9000` |
| Presigned URL "signature does not match" | Presigning must use the public endpoint client |
| No emails | Mailpit running; `MC_SMTP_HOST`/`PORT` for the current mode |
| API refuses to start: "schema is behind" | `pnpm db:migrate` |
| Invalid configuration | `pnpm mcctl config check` lists every problem at once |
