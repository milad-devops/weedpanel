# Database design

PostgreSQL is the source of truth for WeedPanel configuration and identity.

| Table | Purpose |
|---|---|
| clusters | Cluster registry, endpoints, connection mode and last known status |
| cluster_credentials | Encrypted upstream secrets; ciphertext only |
| users | Local/OIDC identities |
| roles / permissions | RBAC catalog |
| user_roles / role_permissions | Many-to-many authorization |
| oidc_providers | OIDC issuer and encrypted client secret |
| cluster_snapshots | Time-series snapshots in JSONB; retention/partitioning later |
| audit_logs | Immutable-style actor/action/outcome history |
| jobs | Background work state and result |

## Important constraints

- Cluster names are case-insensitively unique.
- Credentials are stored separately from endpoint metadata.
- Credential ciphertext includes key version so encryption keys can rotate.
- Audit rows survive user/cluster deletion via `SET NULL`.
- Snapshot table should be partitioned or downsampled for large installations.
- Redis is not the source of truth for jobs; PostgreSQL remains authoritative.

## Follow-up migrations

1. Add `agent_id`, agent public-key fingerprint and enrollment state.
2. Add per-cluster membership and scoped RBAC.
3. Add session/token tables if server-side sessions are selected.
4. Add OIDC state/nonce handling with short TTL.
5. Add retention policies and partitioning for snapshots/audit.
