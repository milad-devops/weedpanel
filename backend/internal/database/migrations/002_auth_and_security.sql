CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at);

ALTER TABLE clusters ADD COLUMN IF NOT EXISTS agent_id UUID;
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS agent_status VARCHAR(20) NOT NULL DEFAULT 'not_configured';

INSERT INTO permissions(id,code,description) VALUES
('10000000-0000-4000-8000-000000000001','clusters.read','View cluster inventory'),
('10000000-0000-4000-8000-000000000002','clusters.write','Create and update cluster registrations'),
('10000000-0000-4000-8000-000000000003','clusters.delete','Delete cluster registrations'),
('10000000-0000-4000-8000-000000000004','clusters.connect','Run cluster connection tests'),
('10000000-0000-4000-8000-000000000005','users.manage','Manage users and roles')
ON CONFLICT(code) DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='super_admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM roles r JOIN permissions p ON p.code IN ('clusters.read','clusters.connect')
WHERE r.name IN ('operator','read_only')
ON CONFLICT DO NOTHING;
