#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(dirname "$(realpath "$0")")")"
if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "Usage: $0 backups/weedpanel-postgres-YYYYMMDDTHHMMSSZ.sql.gz" >&2
  exit 1
fi
echo "WARNING: This imports into the configured WeedPanel database. Use only on a fresh/empty target database before starting the backend."
read -r -p "Continue? [y/N] " answer
[[ "$answer" == "y" || "$answer" == "Y" ]] || exit 1
gzip -dc "$1" | docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
echo "Restore completed. Start the full stack with: docker compose up -d --build"
