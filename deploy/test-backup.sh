#!/usr/bin/env bash
# Uji backup/restore: (a) retensi, (b) kegagalan, (c) siklus backup -> ubah -> restore.
# Tanpa argumen; butuh Docker. Exit 0 bila semua cek lulus.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PG_IMAGE="${PG_IMAGE:-postgres:16}"
ID="natter-bt-$$"
NET="$ID-net"
DB="$ID-db"
TMP="$(mktemp -d)"
FAILS=0

cleanup() {
  docker rm -f "$DB" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  # File dibuat oleh container (uid lain); hapus lewat container bila perlu.
  rm -rf "$TMP" 2>/dev/null || docker run --rm -v "$TMP:/t" "$PG_IMAGE" sh -c 'rm -rf /t/*' >/dev/null 2>&1 || true
  rm -rf "$TMP" 2>/dev/null || true
}
trap cleanup EXIT

pass() { echo "PASS: $*"; }
fail() { echo "FAIL: $*"; FAILS=$((FAILS + 1)); }
# chk <pesan-lulus> <pesan-gagal> <perintah...>
chk() { local ok=$1 bad=$2; shift 2; if "$@"; then pass "$ok"; else fail "$bad"; fi; }

# runpg <cmd...>: jalankan perintah di container postgres:16 dengan deploy/ dan $TMP.
runpg() {
  docker run --rm --network "$NET" \
    -v "$HERE:/deploy:ro" -v "$TMP:/work" \
    -e PGHOST -e PGPORT -e PGUSER -e PGPASSWORD -e PGDATABASE -e BACKUP_DIR -e TZ -e PRUNE_ONLY \
    "$PG_IMAGE" "$@"
}

docker network create "$NET" >/dev/null

# ---------- (a) Retensi ----------
echo "== (a) retensi"
R="$TMP/a"; mkdir -p "$R"
for i in $(seq 1 10); do echo x >"$R/natter-202601$(printf %02d "$i")-0200-daily.dump"; done
for i in $(seq 1 6);  do echo x >"$R/natter-202602$(printf %02d "$i")-0200-weekly.dump"; done
for i in $(seq 1 7);  do echo x >"$R/natter-202603$(printf %02d "$i")-1200-preupdate.dump"; done
touch "$R/notes.txt"
if [ -f "$HERE/backup.sh" ] && PRUNE_ONLY=1 BACKUP_DIR="$R" bash "$HERE/backup.sh" daily >/dev/null; then
  want_d="$(for i in $(seq 4 10); do echo "natter-202601$(printf %02d "$i")-0200-daily.dump"; done)"
  want_w="$(for i in $(seq 3 6);  do echo "natter-202602$(printf %02d "$i")-0200-weekly.dump"; done)"
  want_p="$(for i in $(seq 3 7);  do echo "natter-202603$(printf %02d "$i")-1200-preupdate.dump"; done)"
  got() { find "$R" -maxdepth 1 -name "*-$1.dump" -printf '%f\n' | sort; }
  chk "7 daily terbaru tersisa" "daily: $(got daily | tr '\n' ' ')" [ "$(got daily)" = "$want_d" ]
  chk "4 weekly terbaru tersisa" "weekly: $(got weekly | tr '\n' ' ')" [ "$(got weekly)" = "$want_w" ]
  chk "5 preupdate terbaru tersisa" "preupdate: $(got preupdate | tr '\n' ' ')" [ "$(got preupdate)" = "$want_p" ]
  chk "notes.txt tetap" "notes.txt terhapus" [ -f "$R/notes.txt" ]
else
  fail "backup.sh PRUNE_ONLY gagal/tidak ada"
fi

# ---------- (a2) File 0-byte ----------
echo "== (a2) file 0-byte diabaikan"
Z="$TMP/a2"; mkdir -p "$Z"
for i in $(seq 1 7); do echo x >"$Z/natter-202601$(printf %02d "$i")-0200-daily.dump"; done
: >"$Z/natter-20260110-0200-daily.dump"
PRUNE_ONLY=1 BACKUP_DIR="$Z" bash "$HERE/backup.sh" daily >/dev/null
chk "file 0-byte terhapus" "file 0-byte masih ada" [ ! -e "$Z/natter-20260110-0200-daily.dump" ]
chk "7 daily baik utuh" "daily baik: $(find "$Z" -type f -printf '%f ' | sort)" [ "$(find "$Z" -name '*-daily.dump' | wc -l)" -eq 7 ]

# ---------- (b) Gagal ----------
echo "== (b) kegagalan"
mkdir -p "$TMP/b"
for i in $(seq 1 8); do echo "old$i" >"$TMP/b/natter-202601$(printf %02d "$i")-0200-daily.dump"; done
for i in 1 2; do echo "w$i" >"$TMP/b/natter-202602$(printf %02d "$i")-0200-weekly.dump"; done
for i in 1 2; do echo "p$i" >"$TMP/b/natter-202603$(printf %02d "$i")-1200-preupdate.dump"; done
(cd "$TMP/b" && sha256sum ./* | sort) >"$TMP/b.before"
set +e
PGHOST=nonexistent.invalid PGPORT=5432 PGUSER=x PGPASSWORD=x PGDATABASE=x BACKUP_DIR=/work/b TZ=UTC PRUNE_ONLY="" \
  runpg bash /deploy/backup.sh daily >"$TMP/b.out" 2>"$TMP/b.err"
rc=$?
set -e
chk "exit $rc != 0" "exit 0 saat PGHOST tak terjangkau" [ "$rc" -ne 0 ]
after="$(cd "$TMP/b" && sha256sum ./* | sort)"
chk "12 file lama utuh, tak ada file baru" "isi dir berubah: $(find "$TMP/b" -type f -printf '%f ')" [ "$after" = "$(cat "$TMP/b.before")" ]
chk 'stderr berisi "msg":"backup failed"' "stderr: $(cat "$TMP/b.err")" grep -q '"msg":"backup failed"' "$TMP/b.err"
chk 'stderr berisi "level":"ERROR"' "stderr: $(cat "$TMP/b.err")" grep -q '"level":"ERROR"' "$TMP/b.err"

# ---------- (d) Jadwal cron & DST ----------
echo "== (d) next_run lintas DST"
dst_check() { # <tz> <now-lokal> <harapan-lokal>
  local got
  got="$(TZ=$1 bash -c 'source "$0"; t=$(next_run "$(date -d "$1" +%s)"); date -d "@$t" "+%F %H:%M"' "$HERE/backup-cron.sh" "$2" 2>&1)" || true
  chk "$1 $2 -> $3" "$1 $2 -> '$got' (harap $3)" [ "$got" = "$3" ]
}
dst_check Europe/Berlin    "2026-03-28 23:00" "2026-03-29 03:00"
dst_check America/New_York "2026-03-07 23:00" "2026-03-08 03:00"
dst_check Europe/Berlin    "2026-03-29 01:30" "2026-03-29 03:00"
dst_check Europe/Berlin    "2026-10-24 23:00" "2026-10-25 02:00"
dst_check UTC              "2026-11-02 02:00" "2026-11-03 02:00"
dst_check UTC              "2026-11-02 01:59" "2026-11-02 02:00"

# ---------- (c) Siklus ----------
echo "== (c) siklus"
docker run -d --name "$DB" --network "$NET" -e POSTGRES_USER=natter -e POSTGRES_PASSWORD=pw -e POSTGRES_DB=natter "$PG_IMAGE" >/dev/null
for _ in $(seq 1 60); do
  docker exec "$DB" pg_isready -U natter -d natter -h 127.0.0.1 >/dev/null 2>&1 && break; sleep 1
done
export PGHOST="$DB" PGPORT=5432 PGUSER=natter PGPASSWORD=pw PGDATABASE=natter TZ=UTC PRUNE_ONLY=""
psql_c() { runpg psql -v ON_ERROR_STOP=1 -qAt -c "$1"; }
psql_c "CREATE TABLE t(x int); INSERT INTO t VALUES (1),(2),(3);"
mkdir -p "$TMP/c"; chmod 777 "$TMP/c"
if BACKUP_DIR=/work/c runpg bash /deploy/backup.sh daily >"$TMP/c.out" 2>"$TMP/c.err"; then
  f="$(find "$TMP/c" -maxdepth 1 -name 'natter-*.dump' -printf '%f\n' | head -1)"
  if [ -n "$f" ]; then
    pass "backup dibuat: $f"
    psql_c "DELETE FROM t WHERE x IN (1,2,3); INSERT INTO t VALUES (99);"
    runpg pg_restore --clean --if-exists --no-owner -d natter "/work/c/$f"
    got="$(psql_c "SELECT string_agg(x::text, ',' ORDER BY x) FROM t;")"
    chk "isi setelah restore = 1,2,3" "isi setelah restore = $got" [ "$got" = "1,2,3" ]
  else
    fail "tidak ada file dump"
  fi
else
  fail "backup.sh daily gagal: $(cat "$TMP/c.err")"
fi

echo "== $FAILS kegagalan"
[ "$FAILS" -eq 0 ]
