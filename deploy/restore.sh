#!/usr/bin/env bash
# restore.sh <file.dump> --yes [--no-safety-dump] — DESTRUKTIF: menimpa database natter dari dump.
# Dijalankan di host dari root repo; memakai `docker compose` (hormati COMPOSE_FILE).
set -euo pipefail

file=${1:-}
confirm=${2:-}
safety=${3:-}
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/healthz}"

if [ -z "$file" ]; then
  echo "usage: deploy/restore.sh <file.dump> --yes" >&2; exit 2
fi
if [ ! -f "$file" ]; then
  echo "error: file not found: $file" >&2; exit 2
fi
echo "WARNING: this replaces ALL data in database natter with $file (app will be stopped)." >&2
if [ "$confirm" != "--yes" ]; then
  echo "refusing: pass --yes as second argument to proceed" >&2; exit 2
fi

echo "verifying dump" >&2
if ! docker compose exec -T postgres pg_restore -l <"$file" >/dev/null; then
  echo "error: not a valid pg_dump custom-format file: $file (app untouched)" >&2; exit 1
fi

# Hentikan Watchtower selama restore (jangan sampai memperbarui app di tengah jalan); nyalakan lagi saat keluar.
wt_stopped=0
# shellcheck disable=SC2329  # dipanggil lewat trap
restart_watchtower() {
  if [ "$wt_stopped" = 1 ]; then
    echo "starting watchtower" >&2
    docker compose start watchtower || echo "warning: failed to start watchtower" >&2
  fi
}
trap restart_watchtower EXIT
if docker compose ps --services 2>/dev/null | grep -qx watchtower; then
  echo "stopping watchtower" >&2
  docker compose stop watchtower
  wt_stopped=1
fi

# Safety dump dari kondisi sekarang, sebelum ada yang disentuh.
if [ -n "$(docker compose ps --status running -q app 2>/dev/null)" ]; then
  echo "taking safety dump (preupdate)" >&2
  if ! docker compose exec -T app /app/deploy/backup.sh preupdate; then
    if [ "$safety" = "--no-safety-dump" ]; then
      echo "warning: safety dump failed; continuing (--no-safety-dump)" >&2
    else
      echo "error: safety dump failed; nothing touched. Pass --no-safety-dump as 3rd arg to skip." >&2; exit 1
    fi
  fi
else
  echo "app not running; skipping safety dump" >&2
fi

echo "stopping app" >&2
docker compose stop app
echo "restoring $file" >&2
if ! docker compose exec -T postgres pg_restore --single-transaction --clean --if-exists --no-owner -U natter -d natter <"$file"; then
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
