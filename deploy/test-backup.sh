#!/usr/bin/env bash
# Uji backup/restore: (a) retensi, (b) kegagalan, (c) siklus backup -> ubah -> restore.
# Tanpa argumen; butuh Docker. Exit 0 bila semua cek lulus.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PG_IMAGE="${PG_IMAGE:-postgres:16}"
ID="pmess-bt-$$"
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
for i in $(seq 1 10); do touch "$R/pmess-202601$(printf %02d "$i")-0200-daily.dump"; done
for i in $(seq 1 6);  do touch "$R/pmess-202602$(printf %02d "$i")-0200-weekly.dump"; done
for i in $(seq 1 7);  do touch "$R/pmess-202603$(printf %02d "$i")-1200-preupdate.dump"; done
touch "$R/notes.txt"
if [ -f "$HERE/backup.sh" ] && PRUNE_ONLY=1 BACKUP_DIR="$R" bash "$HERE/backup.sh" daily >/dev/null; then
  want_d="$(for i in $(seq 4 10); do echo "pmess-202601$(printf %02d "$i")-0200-daily.dump"; done)"
  want_w="$(for i in $(seq 3 6);  do echo "pmess-202602$(printf %02d "$i")-0200-weekly.dump"; done)"
  want_p="$(for i in $(seq 3 7);  do echo "pmess-202603$(printf %02d "$i")-1200-preupdate.dump"; done)"
  got() { find "$R" -maxdepth 1 -name "*-$1.dump" -printf '%f\n' | sort; }
  chk "7 daily terbaru tersisa" "daily: $(got daily | tr '\n' ' ')" [ "$(got daily)" = "$want_d" ]
  chk "4 weekly terbaru tersisa" "weekly: $(got weekly | tr '\n' ' ')" [ "$(got weekly)" = "$want_w" ]
  chk "5 preupdate terbaru tersisa" "preupdate: $(got preupdate | tr '\n' ' ')" [ "$(got preupdate)" = "$want_p" ]
  chk "notes.txt tetap" "notes.txt terhapus" [ -f "$R/notes.txt" ]
else
  fail "backup.sh PRUNE_ONLY gagal/tidak ada"
fi

# ---------- (b) Gagal ----------
echo "== (b) kegagalan"
mkdir -p "$TMP/b"
echo old > "$TMP/b/pmess-20260101-0200-daily.dump"
set +e
PGHOST=nonexistent.invalid PGPORT=5432 PGUSER=x PGPASSWORD=x PGDATABASE=x BACKUP_DIR=/work/b TZ=UTC PRUNE_ONLY="" \
  runpg bash /deploy/backup.sh daily >"$TMP/b.out" 2>"$TMP/b.err"
rc=$?
set -e
chk "exit $rc != 0" "exit 0 saat PGHOST tak terjangkau" [ "$rc" -ne 0 ]
n="$(find "$TMP/b" -type f | wc -l)"
chk "tidak ada file baru" "file baru: $(find "$TMP/b" -type f -printf '%f ')" [ "$n" -eq 1 ]
chk "file lama utuh" "file lama berubah" [ "$(cat "$TMP/b/pmess-20260101-0200-daily.dump")" = old ]
chk 'stderr berisi "level":"ERROR"' "stderr: $(cat "$TMP/b.err")" grep -q '"level":"ERROR"' "$TMP/b.err"

# ---------- (c) Siklus ----------
echo "== (c) siklus"
docker run -d --name "$DB" --network "$NET" -e POSTGRES_USER=pmess -e POSTGRES_PASSWORD=pw -e POSTGRES_DB=pmess "$PG_IMAGE" >/dev/null
for _ in $(seq 1 60); do
  docker exec "$DB" pg_isready -U pmess -d pmess -h 127.0.0.1 >/dev/null 2>&1 && break; sleep 1
done
export PGHOST="$DB" PGPORT=5432 PGUSER=pmess PGPASSWORD=pw PGDATABASE=pmess TZ=UTC PRUNE_ONLY=""
psql_c() { runpg psql -v ON_ERROR_STOP=1 -qAt -c "$1"; }
psql_c "CREATE TABLE t(x int); INSERT INTO t VALUES (1),(2),(3);"
mkdir -p "$TMP/c"; chmod 777 "$TMP/c"
if BACKUP_DIR=/work/c runpg bash /deploy/backup.sh daily >"$TMP/c.out" 2>"$TMP/c.err"; then
  f="$(find "$TMP/c" -maxdepth 1 -name 'pmess-*.dump' -printf '%f\n' | head -1)"
  if [ -n "$f" ]; then
    pass "backup dibuat: $f"
    psql_c "DELETE FROM t WHERE x IN (1,2,3); INSERT INTO t VALUES (99);"
    runpg pg_restore --clean --if-exists --no-owner -d pmess "/work/c/$f"
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
