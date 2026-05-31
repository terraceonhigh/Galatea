#!/usr/bin/env bash
# run-a1-live.sh — the Phase A1 LIVE gate for Galatea's libfuse shim.
#
# Builds libgalateafuse.dylib + the passthrough fixture, mounts it through
# Galatea's NFSv4 server (mount_nfs, unprivileged — no root, no kext, no
# FUSE-T), and exercises the A1 structural ops (symlink / readlink / hard link)
# over the real mount, asserting both the client-visible result and the effect
# in the backing store. Tears down unconditionally on exit.
#
# Run from the repo root (or anywhere — paths derive from this script's location):
#   bash shim/libfuse/fixture/run-a1-live.sh
#
# Exit 0 = all checks passed. Nonzero = a check failed (message says which).
set -u

# --- locate the tree ---------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHIM_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"          # shim/libfuse
REPO_ROOT="$(cd "$SHIM_DIR/../.." && pwd)"        # repo root

WORK="$(mktemp -d /tmp/galatea-a1.XXXXXX)"
BACKING="$WORK/backing"
MNT="$WORK/mnt"
mkdir -p "$BACKING" "$MNT"

PTFS_PID=""
PASS=0; FAIL=0
ok()   { echo "  PASS: $1"; PASS=$((PASS+1)); }
bad()  { echo "  FAIL: $1"; FAIL=$((FAIL+1)); }

cleanup() {
  echo "--- teardown ---"
  if [ -n "$PTFS_PID" ] && kill -0 "$PTFS_PID" 2>/dev/null; then
    kill -TERM "$PTFS_PID" 2>/dev/null   # the shim unmounts itself on SIGTERM
    for _ in $(seq 1 20); do kill -0 "$PTFS_PID" 2>/dev/null || break; sleep 0.3; done
  fi
  # belt-and-suspenders: force-unmount if still mounted, then remove the workdir
  if mount | grep -q " $MNT "; then umount -f "$MNT" 2>/dev/null || diskutil unmount force "$MNT" 2>/dev/null; fi
  rm -rf "$WORK" 2>/dev/null
  echo "  workdir removed: $WORK"
}
trap cleanup EXIT

# --- build -------------------------------------------------------------------
echo "--- build ---"
( cd "$REPO_ROOT" && go build -buildmode=c-shared -o "$WORK/libgalateafuse.dylib" ./shim/libfuse ) \
  || { echo "FATAL: dylib build failed"; exit 1; }
cc -D_FILE_OFFSET_BITS=64 -DFUSE_USE_VERSION=26 -I "$SHIM_DIR/include" \
   "$SHIM_DIR/fixture/passthrough.c" -o "$WORK/ptfs" -L "$WORK" -lgalateafuse \
  || { echo "FATAL: fixture compile failed"; exit 1; }
echo "  built dylib + ptfs in $WORK"

# seed a target file in the backing store (appears at the mount as /target.txt)
printf 'body' > "$BACKING/target.txt"

# --- mount -------------------------------------------------------------------
echo "--- mount ---"
DYLD_LIBRARY_PATH="$WORK" "$WORK/ptfs" "$BACKING" "$MNT" &
PTFS_PID=$!
# wait for the mount to come up (seed file becomes visible through it)
READY=0
for _ in $(seq 1 50); do
  if [ "$(cat "$MNT/target.txt" 2>/dev/null)" = "body" ]; then READY=1; break; fi
  kill -0 "$PTFS_PID" 2>/dev/null || { echo "FATAL: ptfs exited before mount came up"; exit 1; }
  sleep 0.3
done
[ "$READY" = 1 ] || { echo "FATAL: mount did not become ready in ~15s"; exit 1; }
echo "  mounted at $MNT (backing: $BACKING)"

# --- A1 checks (over the real NFS mount) ------------------------------------
echo "--- A1 checks ---"

# 1. symlink: create at the mount → real symlink in the backing store.
if ln -s target.txt "$MNT/sym" 2>/dev/null; then
  if [ -L "$BACKING/sym" ]; then ok "symlink created (backing store has a real symlink)"; else bad "symlink: backing entry is not a symlink"; fi
else bad "symlink: ln -s at the mount failed"; fi

# 2. readlink: read the target back through the mount.
GOT="$(readlink "$MNT/sym" 2>/dev/null || true)"
[ "$GOT" = "target.txt" ] && ok "readlink returns 'target.txt'" || bad "readlink returned '$GOT' (want target.txt)"

# 3. symlink resolves: reading through the link yields the target's content.
GOT="$(cat "$MNT/sym" 2>/dev/null || true)"
[ "$GOT" = "body" ] && ok "symlink resolves (cat through link = 'body')" || bad "cat through symlink = '$GOT' (want body)"

# 4. absolute target round-trips verbatim (the path-normalizer concern).
if ln -s /etc/hosts "$MNT/abs" 2>/dev/null; then
  GOT="$(readlink "$MNT/abs" 2>/dev/null || true)"
  [ "$GOT" = "/etc/hosts" ] && ok "absolute symlink target round-trips ('/etc/hosts')" || bad "absolute target = '$GOT' (want /etc/hosts)"
else bad "absolute symlink: ln -s failed"; fi

# 5. hard link: link at the mount → second name in the backing store, nlink 2.
if ln "$MNT/target.txt" "$MNT/hard" 2>/dev/null; then
  GOT="$(cat "$MNT/hard" 2>/dev/null || true)"
  NLINK="$(stat -f '%l' "$BACKING/target.txt" 2>/dev/null || echo '?')"
  [ "$GOT" = "body" ] && ok "hard link content = 'body'" || bad "hard link content = '$GOT'"
  [ "$NLINK" = "2" ] && ok "hard link nlink = 2" || bad "nlink = $NLINK (want 2)"
else bad "hard link: ln at the mount failed"; fi

# 6. utimens via `touch -t` — an explicit timestamp (SET_TO_CLIENT_TIME). This
#    is the path the server fully implements; expect the mtime to change to 2000.
if touch -t 200001020304.05 "$MNT/target.txt" 2>/dev/null; then
  YEAR="$(stat -f '%Sm' -t '%Y' "$MNT/target.txt" 2>/dev/null || true)"
  [ "$YEAR" = "2000" ] && ok "touch -t sets mtime (year 2000 through the mount)" || bad "touch -t: mtime year = '$YEAR' (want 2000)"
  BYEAR="$(stat -f '%Sm' -t '%Y' "$BACKING/target.txt" 2>/dev/null || true)"
  [ "$BYEAR" = "2000" ] && ok "touch -t effect lands in the backing store" || bad "touch -t: backing mtime year = '$BYEAR' (want 2000)"
else bad "touch -t at the mount failed"; fi

# 7. plain `touch` (current time) — INFORMATIONAL, not a pass/fail. macOS may
#    send SET_TO_SERVER_TIME for this, which the server decodes but does NOT
#    apply (no wall clock — server-time is a deferred architecture decision). So
#    a no-op here is the expected, documented gap; this probe just reveals which
#    path macOS actually uses, so we know whether server-time is worth building.
BEFORE="$(stat -f '%m' "$MNT/target.txt" 2>/dev/null || echo 0)"
touch "$MNT/target.txt" 2>/dev/null || true
AFTER="$(stat -f '%m' "$MNT/target.txt" 2>/dev/null || echo 0)"
if [ "$AFTER" != "$BEFORE" ]; then
  echo "  INFO: plain touch changed mtime ($BEFORE → $AFTER) — macOS sends SET_TO_CLIENT_TIME"
else
  echo "  INFO: plain touch left mtime unchanged — macOS likely sends SET_TO_SERVER_TIME (unimplemented by design; the expected gap)"
fi

# --- verdict -----------------------------------------------------------------
echo "--- result: $PASS passed, $FAIL failed ---"
[ "$FAIL" = 0 ]
