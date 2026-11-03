#!/usr/bin/env sh
set -eu
if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env. Generate secrets, edit it, then rerun this script."
  echo "APP_ENCRYPTION_KEY: openssl rand -base64 32"
  echo "SESSION_SECRET:     openssl rand -hex 32"
  exit 1
fi
docker compose up --build -d
docker compose ps
