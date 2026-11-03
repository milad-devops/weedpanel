
# API contract v1

Base path: `/api/v1`. JSON request/response.

## Authentication
- `POST /auth/register`: one-time first-user bootstrap; first user becomes `super_admin`.
- `POST /auth/login`: email/password; creates 12-hour HttpOnly SameSite=Lax session cookie.
- `POST /auth/logout`: invalidates session.
- `GET /auth/me`: current identity.

Passwords are bcrypt-hashed. Session tokens are random, only SHA-256 hashes are stored.
This milestone does not yet implement OIDC Authorization Code + PKCE or CSRF token
validation; use only on localhost/trusted development networks.

## Clusters
All cluster routes require a valid session and permission:
- `GET /clusters` — `clusters.read`
- `POST /clusters` — `clusters.write`
- `GET /clusters/{id}` — `clusters.read`
- `PATCH /clusters/{id}` — `clusters.write`
- `DELETE /clusters/{id}` — `clusters.delete`
- `POST /clusters/{id}/connection-test` — `clusters.connect`
- `POST /clusters/{id}/credentials` — `clusters.write`

Cluster request:
```json
{
  "name": "production-eu",
  "description": "Primary cluster",
  "connection_mode": "direct",
  "master_urls": ["http://127.0.0.1:9333"],
  "filer_url": "http://127.0.0.1:8888",
  "s3_url": "http://127.0.0.1:8333",
  "prometheus_url": ""
}
```

Connection test performs a bounded HTTP GET to `/healthz` on each configured Master URL.
Credential request: `{ "type": "s3_access_key", "secret": "..." }`.
Supported types: `s3_access_key`, `s3_secret_key`, `filer_auth`, `master_auth`.
Credential values are encrypted using AES-256-GCM and never returned by the API.

## System
- `GET /system/info`
- `GET /healthz`
- `GET /readyz`

## Planned / not yet implemented
- OIDC endpoints/provider configuration
- Per-cluster resource scoping in RBAC
- Agent enrollment/channel
- Signed S3 credentials and bucket/object mutations
- Prometheus query adapter, metrics history and dashboards
- User/role mutation APIs, password reset and session rotation
- CSRF token strategy and login rate limiting

## Live cluster operations (authenticated)

- `GET /api/v1/clusters/{id}/live?section=topology|storage|volumes|s3` proxies read-only status requests to the registered Master or S3 endpoint.
- `GET /api/v1/clusters/{id}/files?path=/` lists a Filer path.
- `POST /api/v1/clusters/{id}/files?path=/&name=file.bin` uploads a raw request body to the Filer (64 MiB request limit).
- `POST /api/v1/clusters/{id}/directories?path=/&name=folder` creates a directory.
- `GET /api/v1/clusters/{id}/file?path=/file.bin` streams a file download (256 MiB response cap).
- `DELETE /api/v1/clusters/{id}/files?path=/file.bin` deletes a Filer path.
- `GET /api/v1/admin/users` lists users and assigned roles for administrators.

All operations require an authenticated session and the corresponding existing permission. Filer write operations are cluster mutations; they never delete or modify the WeedPanel database.

## Prometheus monitoring

`GET /api/v1/clusters/{id}/metrics?range=1h` requires `clusters.read` and a configured `prometheus_url` on the cluster. Supported ranges are `1h`, `6h`, `24h`, and `7d`. The API queries Prometheus `/api/v1/query_range` for `up`, `process_resident_memory_bytes`, `go_goroutines`, and `rate(process_cpu_seconds_total[5m])`. Responses are live query results; WeedPanel does not persist or aggregate metric history itself. The Prometheus server must retain the requested time window.
