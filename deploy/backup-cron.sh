#!/usr/bin/env bash
# backup-cron.sh — loop: tidur sampai 02:00 berikutnya (waktu $TZ), lalu backup.sh daily.
set -euo pipefail
export TZ="${TZ:-UTC}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AT="${BACKUP_AT:-02:00}"

log() {
  local level=$1; shift
  local line
  line="{\"time\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"$level\",\"msg\":\"$1\"${2:+,$2}}"
  if [ "$level" = ERROR ]; then echo "$line" >&2; else echo "$line"; fi
}

while true; do
  now=$(date +%s)
  next=$(date -d "today $AT" +%s)
  [ "$next" -gt "$now" ] || next=$(date -d "tomorrow $AT" +%s)
  log INFO "backup scheduled" "\"next\":\"$(date -d "@$next" +%Y-%m-%dT%H:%M:%S%z)\",\"sleep_s\":$((next - now))"
  sleep "$((next - now))"
  if "$HERE/backup.sh" daily; then
    log INFO "backup run finished" "\"ok\":true"
  else
    log ERROR "backup run failed" "\"ok\":false"
  fi
done
