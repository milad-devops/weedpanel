#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(dirname "$(realpath "$0")")")"

[[ -f .env ]] || { echo "Missing .env. Refusing to generate new secrets for an existing installation." >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Docker is required." >&2; exit 1; }
docker compose version >/dev/null 2>&1 || { echo "Docker Compose plugin is required." >&2; exit 1; }
docker network inspect separ-deploy_web >/dev/null 2>&1 || { echo "Docker network separ-deploy_web is missing; check the existing Caddy network before updating." >&2; exit 1; }
docker compose ps --services --status running | grep -qx postgres || { echo "PostgreSQL is not running. Start the existing stack first; this script will not initialize a new database." >&2; exit 1; }

chmod 600 .env
echo "Creating a PostgreSQL backup before updating..."
./scripts/backup.sh

echo "Validating Compose configuration..."
docker compose config >/dev/null

echo "Building and updating application containers. Persistent volumes are left untouched."
docker compose up -d --build

docker compose ps
