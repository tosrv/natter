#!/usr/bin/env bash
# backup.sh <daily|preupdate> — pg_dump -Fc ke $BACKUP_DIR lalu retensi.
# Env: PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE BACKUP_DIR(/backups) TZ(UTC)
#      PRUNE_ONLY=1 (lewati dump, hanya retensi; untuk test)
#      KEEP_DAILY(7) KEEP_WEEKLY(4) KEEP_PREUPDATE(5)
set -euo pipefail

export TZ="${TZ:-UTC}"
BACKUP_DIR="${BACKUP_DIR:-/backups}"
KEEP_DAILY="${KEEP_DAILY:-7}"
KEEP_WEEKLY="${KEEP_WEEKLY:-4}"
KEEP_PREUPDATE="${KEEP_PREUPDATE:-5}"

json_escape() {
  local s=$1
  s=${s//\\/\\\\}; s=${s//\"/\\\"}; s=${s//$'\n'/ }; s=${s//$'\r'/ }; s=${s//$'\t'/ }
  printf '%s' "$s"
}
# log <LEVEL> <msg> [key value]...
log() {
  local level=$1 msg=$2 line
  shift 2
  line="{\"time\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"$level\",\"msg\":\"$(json_escape "$msg")\""
  while [ $# -ge 2 ]; do
    line+=",\"$1\":\"$(json_escape "$2")\""
    shift 2
  done
  line+="}"
  if [ "$level" = ERROR ]; then echo "$line" >&2; else echo "$line"; fi
}

# prune <kind> <keep>: hapus file kind tertua di luar <keep> terbaru (urut nama/timestamp).
prune() {
  local kind=$1 keep=$2 f
  local re="^pmess-([0-9]{8})-([0-9]{4})-${kind}(-([0-9]+))?\\.dump$"
  local -a entries=()
  for f in "$BACKUP_DIR"/pmess-*-"$kind"*.dump; do
    [ -f "$f" ] || continue
    f=${f##*/}
    if [[ $f =~ $re ]]; then
      entries+=("${BASH_REMATCH[1]}${BASH_REMATCH[2]} ${BASH_REMATCH[4]:-1} $f")
    fi
  done
  [ "${#entries[@]}" -gt "$keep" ] || return 0
  while read -r _ _ f; do
    rm -f -- "${BACKUP_DIR:?}/$f"
    log INFO "backup pruned" kind "$kind" file "$f"
  done < <(printf '%s\n' "${entries[@]}" | sort -k1,1n -k2,2n | head -n $(( ${#entries[@]} - keep )))
}

prune_all() {
  # File 0-byte = reservasi yang terputus; bukan backup, jangan dihitung.
  local z
  for z in "$BACKUP_DIR"/pmess-*.dump; do
    [ -f "$z" ] && [ ! -s "$z" ] || continue
    rm -f -- "$z"
    log INFO "empty backup file removed" file "${z##*/}"
  done
  prune daily "$KEEP_DAILY"
  prune weekly "$KEEP_WEEKLY"
  prune preupdate "$KEEP_PREUPDATE"
}

mode=${1:-}
case "$mode" in
  daily|preupdate) ;;
  *) log ERROR "usage: backup.sh <daily|preupdate>" arg "$mode"; exit 2 ;;
esac

if [ ! -d "$BACKUP_DIR" ]; then
  log ERROR "backup dir missing" dir "$BACKUP_DIR"; exit 1
fi

if [ "${PRUNE_ONLY:-}" = 1 ]; then
  prune_all
  exit 0
fi

kind=$mode
if [ "$mode" = daily ] && [ "$(date +%u)" = 7 ]; then kind=weekly; fi

stamp="$(date +%Y%m%d-%H%M)"
base="pmess-$stamp-$kind"
# Reservasi atomik (noclobber = O_EXCL): aman bila dua proses jalan di menit yang sama.
target="$BACKUP_DIR/$base.dump"
n=2
while ! (set -o noclobber; : >"$target") 2>/dev/null; do
  if [ "$n" -gt 100 ]; then log ERROR "cannot reserve backup file" file "${target##*/}"; exit 1; fi
  target="$BACKUP_DIR/$base-$n.dump"; n=$((n + 1))
done
partial="$target.partial"

# Bila gagal: hapus file parsial dan file reservasi kosong.
cleanup_partial() { rm -f -- "$partial" "$target"; }
trap cleanup_partial EXIT

start=$(date +%s)
if ! err="$(pg_dump -Fc -f "$partial" 2>&1)"; then
  log ERROR "backup failed" kind "$kind" error "$err"
  exit 1
fi
mv -- "$partial" "$target"
trap - EXIT
log INFO "backup ok" kind "$kind" file "${target##*/}" bytes "$(wc -c <"$target" | tr -d ' ')" duration_s "$(( $(date +%s) - start ))"

prune_all
