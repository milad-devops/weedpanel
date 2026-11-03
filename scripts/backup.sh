#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(dirname "$(realpath "$0")")")"
mkdir -p backups
stamp=$(date -u +%Y%m%dT%H%M%SZ)
out="backups/weedpanel-postgres-${stamp}.sql.gz"
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges' | gzip -9 > "$out"
echo "Backup created: $out"
