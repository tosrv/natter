#!/usr/bin/env bash
# restore.sh <file.dump> --yes — DESTRUKTIF: menimpa database pmess dari dump.
# Dijalankan di host dari root repo; memakai `docker compose` (hormati COMPOSE_FILE).
set -euo pipefail

file=${1:-}
confirm=${2:-}
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/healthz}"

if [ -z "$file" ]; then
  echo "usage: deploy/restore.sh <file.dump> --yes" >&2; exit 2
fi
if [ ! -f "$file" ]; then
  echo "error: file not found: $file" >&2; exit 2
fi
echo "WARNING: this replaces ALL data in database pmess with $file (app will be stopped)." >&2
if [ "$confirm" != "--yes" ]; then
  echo "refusing: pass --yes as second argument to proceed" >&2; exit 2
fi

echo "verifying dump" >&2
if ! docker compose exec -T postgres pg_restore -l <"$file" >/dev/null; then
  echo "error: not a valid pg_dump custom-format file: $file (app untouched)" >&2; exit 1
fi
echo "stopping app" >&2
docker compose stop app
echo "restoring $file" >&2
if ! docker compose exec -T postgres pg_restore --clean --if-exists --no-owner -U pmess -d pmess <"$file"; then
  echo "error: pg_restore failed; app left stopped" >&2; exit 1
fi
echo "starting app" >&2
docker compose start app
for _ in $(seq 1 60); do
  if curl -fsS -o /dev/null "$HEALTH_URL"; then
    echo "restore ok: /healthz 200" >&2; exit 0
  fi
  sleep 1
done
echo "error: /healthz not OK after 60s" >&2
exit 1
