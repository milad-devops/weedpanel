# Changelog

## 0.3.0 — Functional management baseline

- Replaced inert navigation items with working page navigation.
- Added cluster search, edit, delete, and connection-test controls.
- Added live Master topology, directory status, and volume status views.
- Added Filer directory listing, folder creation, raw file upload, file download, and delete operations.
- Added a read-only view of the configured S3 endpoint response.
- Added administrator user/role listing and a functional account/theme settings page.
- Added authenticated backend endpoints for live status, Filer operations, and administrator user listing.
- Added an update script that backs up PostgreSQL before rebuilding and never removes Compose volumes.
- Attached the frontend to the existing `separ-deploy_web` network for the existing Caddy container.

### Not included yet

Prometheus time-series charts/history, signed S3 credentials and bucket/object mutations, user/role editing, profile/password changes, OIDC, agent mode, CSRF protection, and rate limiting remain future work.

## 0.4.0
- Added authenticated Prometheus range-query API per cluster.
- Added monitoring cards and trend charts for scrape target state, process memory, Go goroutines and CPU usage.
- Added 1h, 6h, 24h and 7d monitoring windows.
