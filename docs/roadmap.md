# Roadmap

## Release 0.3.0 — functional management baseline
- [x] React + TypeScript dashboard and theme toggle
- [x] Docker Compose: frontend, Go API, PostgreSQL, Redis
- [x] PostgreSQL migrations, one-time admin bootstrap, login/logout, HttpOnly sessions
- [x] Initial RBAC permission checks
- [x] Cluster create, read, update, delete and Master connection tests
- [x] Live Master topology status (`/cluster/status`)
- [x] Live directory and volume inventory (`/dir/status`, `/vol/status`)
- [x] Filer directory browsing, folder creation, file upload/download/delete
- [x] Read-only S3 endpoint response view
- [x] Administrator user/role directory (read-only)
- [ ] S3 SigV4 credential integration, bucket/object CRUD
- [ ] Prometheus metrics collection, history, charts and alerts
- [ ] User creation, role editing and cluster-scoped RBAC UI
- [ ] Account profile editing and password rotation
- [ ] OIDC Authorization Code + PKCE, CSRF protection and rate limiting
- [ ] Agent connection and outbound mTLS
- [ ] Integration and end-to-end tests against supported SeaweedFS versions

## Release 0.4.0 — Prometheus monitoring
- [x] Query Prometheus range API for target health, process memory, Go goroutines and CPU usage
- [x] Selectable 1h, 6h, 24h and 7d time windows
- [x] Live monitoring cards with compact trend charts
- [ ] Metric discovery and SeaweedFS-specific metric profiles
- [ ] Alert rules, thresholds and notifications

## Next milestones
1. S3 SigV4 credentials and bucket/object operations.
2. Complete user/role administration and account settings.
3. OIDC, CSRF/rate limiting and agent-based connectivity.
