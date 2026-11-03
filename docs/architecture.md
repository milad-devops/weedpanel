# WeedPanel architecture

## Architecture decision

Start as a modular monolith. Keep domain boundaries explicit so high-load or independently
scalable modules can be extracted later without starting with distributed-systems overhead.

```text
Browser
  |
  v
Frontend (React / TypeScript) -- same-origin /api
  |
  v
WeedPanel API (Go)
  |---- AuthN / AuthZ (local + OIDC; next milestone)
  |---- Cluster registry
  |---- Adapter interfaces
  |       |---- SeaweedFS Master HTTP API
  |       |---- SeaweedFS Filer HTTP API
  |       |---- S3-compatible API
  |       |---- Prometheus HTTP API
  |       `---- optional outbound Agent channel (planned)
  |
  |---- PostgreSQL (configuration, RBAC, audit, snapshots)
  `---- Redis (planned job coordination/cache)
```

## Adapter boundaries

- Master adapter: topology, data centers, racks, volume-server and volume status.
- Filer adapter: namespace browsing, metadata and streaming file operations.
- S3 adapter: bucket/object operations through the S3-compatible API.
- Metrics adapter: query an existing Prometheus or a configured internal Prometheus.
- Agent adapter: outbound authenticated connection for restricted networks; never expose
  an unauthenticated agent control port.

SeaweedFS documents separate Filer, Master and Volume HTTP surfaces. The Filer is the
preferred namespace API; S3 is preferred for S3 object workflows. Do not assume that
all SeaweedFS versions expose identical admin endpoints. Adapters must be version-aware
and capability-driven.

## Connectivity modes

1. Direct: WeedPanel backend reaches configured Master/Filer/S3/Prometheus endpoints.
2. Agent: a cluster-side connector establishes an outbound mutually authenticated channel.
3. Hybrid: choose per cluster and optionally per capability.

## Failure isolation

Every cluster operation has a deadline, bounded concurrency and independent health state.
A timeout or malformed response from one cluster must not block the global overview.
Long-running work is queued and reported as jobs. File transfer paths must stream data
and must not buffer whole objects in API memory.

## Security boundaries

- No browser-to-SeaweedFS direct access for management operations.
- Encrypt secrets with an application key backed by a deployment secret manager.
- TLS for all remote connections; mTLS for agents.
- RBAC checks at the API boundary and again in domain service methods.
- Audit all state-changing actions.
- Destructive operations disabled until explicit capability checks, confirmations and tests exist.


## Monitoring data flow (0.4.0)

The authenticated API reads the Prometheus URL stored on a cluster and issues bounded range queries for a fixed set of operational metrics. Prometheus remains the source of truth and stores the time series; WeedPanel does not create a second metrics database. The browser receives normalized series and renders short trend charts.
