# infra/monitoring

Dashboards and alert rules for production monitoring (see
[OPERATIONS.md](../../docs/architecture/OPERATIONS.md)). Locally, `pnpm dev --observability`
starts Grafana LGTM (http://localhost:3001) and the API/worker export traces to it; metrics are
exposed on each process's admin listener (`/metrics`).
