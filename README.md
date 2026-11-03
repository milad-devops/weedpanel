# WeedPanel

Open-source, multi-cluster management console for SeaweedFS.

> Initial foundation scaffold. The current milestone provides the application shell,
> PostgreSQL-backed cluster registry, health endpoints, and Docker Compose deployment.
> Destructive SeaweedFS operations are intentionally not implemented.

## Stack

- Frontend: React + TypeScript + Vite
- Backend: Go `net/http` + PostgreSQL (`pgx`)
- Runtime: Docker Compose; Redis is provisioned for the upcoming job/coordination layer
- API: `/api/v1`
- UI: Dark/light theme toggle

## Quick start

```bash
cp .env.example .env
# Change POSTGRES_PASSWORD and APP_ENCRYPTION_KEY before exposing this service.
docker compose up --build -d
docker compose ps
```

Open http://localhost:8080

API:
- `GET /healthz` — process health
- `GET /readyz` — database readiness
- `GET /api/v1/system/info` — application metadata
- `GET /api/v1/clusters` — registered clusters
- `POST /api/v1/clusters` — register a cluster

Example:

```bash
curl -s http://localhost:8080/api/v1/system/info
curl -s http://localhost:8080/api/v1/clusters
```

## Development

Backend:

```bash
cd backend
go mod tidy
go run ./cmd/api
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

For local development, set `DATABASE_URL` in `backend/.env` or export it in the shell.
The frontend Vite dev server proxies `/api`, `/healthz`, and `/readyz` to `http://localhost:8081`.

## Architecture

See:
- [Architecture](docs/architecture.md)
- [API contract](docs/api.md)
- [Database design](docs/database.md)
- [Roadmap](docs/roadmap.md)

## Security status

This is an early development scaffold, not production-ready. Authentication/OIDC,
credential encryption, RBAC enforcement, agent transport, rate limiting, CSRF
protection, and full audit coverage are planned before production exposure.
Do not publish the service to an untrusted network yet.

## License

Apache-2.0. See [LICENSE](LICENSE).
