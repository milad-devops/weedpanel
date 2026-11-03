#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(dirname "$(realpath "$0")")")"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed. Install Docker Engine + Compose plugin first." >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin is missing." >&2
  exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate secrets." >&2
  exit 1
fi
if ! docker network inspect separ-deploy_web >/dev/null 2>&1; then
  echo "Docker network separ-deploy_web is missing. Attach the existing Caddy container to this network before deploying." >&2
  exit 1
fi

if [[ ! -f .env ]]; then
  umask 077
  cat > .env <<ENV
APP_ENV=production
APP_LOG_LEVEL=info
HTTP_PORT=18080
POSTGRES_DB=weedpanel
POSTGRES_USER=weedpanel
POSTGRES_PASSWORD=$(openssl rand -hex 32)
APP_ENCRYPTION_KEY=$(openssl rand -base64 32)
SESSION_SECRET=$(openssl rand -hex 48)
ENV
  chmod 600 .env
  echo "Created .env with unique secrets. Keep it private and back it up securely."
else
  echo "Using existing .env; it will not be overwritten."
fi

docker compose config >/dev/null
docker compose up -d --build

echo
echo "WeedPanel containers started. Check: docker compose ps"
echo "Local endpoint: http://127.0.0.1:$(grep '^HTTP_PORT=' .env | cut -d= -f2)"
echo "Caddy site: weed.nuviraa.net -> 127.0.0.1:8080"
