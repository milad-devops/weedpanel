CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    connection_mode VARCHAR(16) NOT NULL DEFAULT 'direct'
        CHECK (connection_mode IN ('direct', 'agent')),
    master_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    filer_url TEXT NOT NULL DEFAULT '',
    s3_url TEXT NOT NULL DEFAULT '',
    prometheus_url TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'unknown'
        CHECK (status IN ('unknown', 'healthy', 'degraded', 'unreachable')),
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS clusters_name_unique ON clusters (LOWER(name));

CREATE TABLE IF NOT EXISTS cluster_credentials (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    credential_type VARCHAR(32) NOT NULL,
    secret_ciphertext BYTEA NOT NULL,
    key_version SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(320) NOT NULL UNIQUE,
    display_name VARCHAR(160) NOT NULL DEFAULT '',
    password_hash TEXT,
    auth_provider VARCHAR(16) NOT NULL DEFAULT 'local'
        CHECK (auth_provider IN ('local', 'oidc')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY,
    name VARCHAR(64) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY,
    code VARCHAR(120) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS cluster_snapshots (
    id BIGSERIAL PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    health VARCHAR(16) NOT NULL,
    payload JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS cluster_snapshots_cluster_time
    ON cluster_snapshots (cluster_id, collected_at DESC);

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    cluster_id UUID REFERENCES clusters(id) ON DELETE SET NULL,
    action VARCHAR(160) NOT NULL,
    resource_type VARCHAR(80) NOT NULL DEFAULT '',
    resource_id VARCHAR(160) NOT NULL DEFAULT '',
    outcome VARCHAR(24) NOT NULL,
    remote_addr INET,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS audit_logs_created_at ON audit_logs (created_at DESC);

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY,
    cluster_id UUID REFERENCES clusters(id) ON DELETE SET NULL,
    job_type VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    result JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS oidc_providers (
    id UUID PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    issuer_url TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_ciphertext BYTEA NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT ARRAY['openid','profile','email'],
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO roles (id, name, description) VALUES
('00000000-0000-4000-8000-000000000001', 'super_admin', 'Full platform administration'),
('00000000-0000-4000-8000-000000000002', 'cluster_admin', 'Manage assigned clusters'),
('00000000-0000-4000-8000-000000000003', 'operator', 'Operate assigned clusters'),
('00000000-0000-4000-8000-000000000004', 'read_only', 'Read-only access')
ON CONFLICT (name) DO NOTHING;
