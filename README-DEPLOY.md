# WeedPanel — deployment on weed.nuviraa.net

This package is configured to run behind the Caddy instance already installed on the host. It does **not** install, replace, or run another Caddy instance and does not bind ports 80/443. Docker publishes only `127.0.0.1:18080` for the frontend; PostgreSQL and Redis remain private on the Compose network.

## 1. DNS and host prerequisites

- Point the `A` record for `weed.nuviraa.net` to this server's public IPv4 address. Add an `AAAA` record only if IPv6 is correctly routed to this host.
- Make sure the existing Caddy instance owns ports 80/443 and can obtain/renew certificates. Do not change its other site blocks.
- Install Docker Engine, the Docker Compose plugin, and `openssl`.

## 2. Upload and start

Extract this archive on the new server, for example to `/opt/weedpanel`, then:

```bash
cd /opt/weedpanel
chmod +x scripts/*.sh
./scripts/deploy.sh
sudo docker compose ps
curl -fsS http://127.0.0.1:18080/healthz
```

`deploy.sh` creates a private `.env` with fresh secrets on first run. If you are migrating an existing database, keep the source `.env` values for `APP_ENCRYPTION_KEY`, `SESSION_SECRET`, and PostgreSQL credentials instead of generating new ones. In particular, **APP_ENCRYPTION_KEY must remain identical** to decrypt any credentials encrypted by the old installation.

## 3. Add the site to the existing Caddy config

Append the contents of `Caddyfile.weedpanel` to the existing Caddyfile, or add it to an already-imported site snippets directory. Do not overwrite the existing Caddyfile. The block below assumes the existing `(cf_tls)` certificate snippet and that the Caddy container is attached to Docker network `separ-deploy_web`. The Compose file attaches the frontend to that existing network. The block is:

```caddy
weed.nuviraa.net {
    encode zstd gzip
    import cf_tls
    reverse_proxy frontend:80
}
```

Validate and reload using the service's actual Caddyfile path:

```bash
docker exec separ-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec separ-caddy caddy reload --config /etc/caddy/Caddyfile
```

If your existing Caddyfile is stored elsewhere, substitute its real path. Caddy will manage HTTPS for this hostname, provided DNS and inbound ports 80/443 are correct.

## 4. Migrating the existing database

On the **old** host, from the old project directory:

```bash
./scripts/backup.sh
```

Copy the resulting `backups/weedpanel-postgres-*.sql.gz` to the new host's project directory. On the new host, copy the old host's `.env` securely into the project directory **before starting any containers**. Preserve at least `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `APP_ENCRYPTION_KEY`, and `SESSION_SECRET`; the encryption key must be identical to decrypt stored cluster credentials. If `deploy.sh` has already generated a new `.env`, replace it with the old one first. Then start only the database and Redis:

```bash
chmod 600 .env
docker compose up -d postgres redis
```

Then restore the dump **before starting backend/frontend**:

```bash
./scripts/restore.sh backups/weedpanel-postgres-YYYYMMDDTHHMMSSZ.sql.gz
```

After restore, start/rebuild the application using that same `.env`:

```bash
docker compose up -d --build
```

For the safest migration, copy the old `.env` securely (permissions `600`) rather than using newly generated secrets. Do not expose it or commit it to Git. The Postgres dump preserves users, roles, sessions, cluster records, and encrypted credentials. Redis is not used for persistent application records in this version.

## 5. First-time setup on an empty database

The registration UI is intentionally hidden. The backend still permits exactly the first account while the users table is empty; all later public registration attempts are rejected. For a truly new/empty database only, create the first administrator with:

```bash
curl -i -X POST https://weed.nuviraa.net/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"REPLACE_WITH_A_LONG_PASSWORD","display_name":"Administrator"}'
```

Do not run this when you have restored the existing database; sign in with the existing administrator instead. The endpoint is bootstrap-only, not a replacement for preserving your database.

## 6. Operations

```bash
docker compose ps
docker compose logs -f --tail=100 backend frontend
./scripts/backup.sh
```

Do not run `docker compose down -v` on a live installation: it deletes the database and Redis volumes.

## 7. Updating an existing installation safely

Extract the new release over the existing `/opt/weedpanel` directory without deleting `.env` or the `backups/` directory. The release includes `scripts/update-existing.sh`; run:

```bash
cd /opt/weedpanel
./scripts/update-existing.sh
```

The script requires the existing `.env`, checks the existing Caddy Docker network and running PostgreSQL, creates a PostgreSQL dump, then runs `docker compose up -d --build`. It does **not** run `down`, `down -v`, recreate the database volume intentionally, or reset administrator accounts and cluster records. Keep the existing `.env` unchanged, especially `POSTGRES_*`, `APP_ENCRYPTION_KEY`, and `SESSION_SECRET`.

The File Manager supports Filer directory browsing, folder creation, file upload/download, and deletion. Deletion is destructive on the selected SeaweedFS cluster; confirm the selected cluster and path before using it. S3 view currently performs an unauthenticated read of the configured S3 endpoint; signed S3 credentials and bucket mutations are not implemented in this release. Monitoring queries the configured Prometheus server for scrape target state, process memory, Go goroutines, and CPU usage over selectable 1h, 6h, 24h, and 7d windows. The Prometheus server must already be scraping the relevant SeaweedFS services and retain the selected time range. Administration currently lists users and roles; user creation and role editing are not implemented yet.
