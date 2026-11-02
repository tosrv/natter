#!/usr/bin/env bash
# backup-cron.sh — loop: tidur sampai BACKUP_AT (default 02:00) berikutnya di $TZ, lalu backup.sh daily.
# Bisa di-source (mis. oleh test) untuk memakai next_run tanpa menjalankan loop.
set -euo pipefail
export TZ="${TZ:-UTC}"
AT="${BACKUP_AT:-02:00}"
FALLBACK_AT="${BACKUP_FALLBACK_AT:-03:00}"

log() {
  local level=$1 line
  shift
  line="{\"time\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"$level\",\"msg\":\"$1\"${2:+,$2}}"
  if [ "$level" = ERROR ]; then echo "$line" >&2; else echo "$line"; fi
}

# at_on <YYYY-MM-DD>: epoch untuk AT pada hari itu; bila jam itu tak ada (DST maju) pakai FALLBACK_AT.
at_on() {
  date -d "$1 $AT" +%s 2>/dev/null || date -d "$1 $FALLBACK_AT" +%s
}

# next_run [now_epoch]: epoch jadwal berikutnya (> now). Default now = NOW_OVERRIDE atau sekarang.
next_run() {
  local now=${1:-${NOW_OVERRIDE:-$(date +%s)}} day t
  day=$(date -d "@$now" +%F)
  t=$(at_on "$day")
  if [ "$t" -le "$now" ]; then
    # Siang hari tidak pernah terkena transisi DST, jadi aman untuk +1 hari.
    day=$(date -d "$day 12:00 +1 day" +%F)
    t=$(at_on "$day")
  fi
  echo "$t"
}

# Di-source? Berhenti di sini (jangan jalankan loop).
(return 0 2>/dev/null) && return 0

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
while true; do
  now=$(date +%s)
  if ! next=$(next_run "$now") || [ -z "$next" ]; then
    log ERROR "backup schedule computation failed" "\"retry_s\":60"
    sleep 60
    continue
  fi
  log INFO "backup scheduled" "\"next\":\"$(date -d "@$next" +%Y-%m-%dT%H:%M:%S%z)\",\"sleep_s\":$((next - now))"
  sleep "$((next - now))"
  if "$HERE/backup.sh" daily; then
    log INFO "backup run finished" "\"ok\":true"
  else
    log ERROR "backup run failed" "\"ok\":false"
  fi
done
