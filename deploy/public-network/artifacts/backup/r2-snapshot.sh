#!/usr/bin/env bash
#
# Nightly snapshot of a Quidnug node's data_dir to Cloudflare R2.
#
# Strategy:
#   1. Quiesce via `quidnug-cli snapshot begin` (flushes pending
#      transactions into a block, fsyncs, returns a snapshot ref).
#   2. tar+zstd the data_dir to a local tempfile.
#   3. `rclone copy` the tarball into R2.
#   4. Verify upload size matches local size.
#   5. `quidnug-cli snapshot end` (resumes normal block production).
#
# Retention is handled by R2 object-lifecycle rules:
#   - 14 days in Standard
#   - 12 weeks in Infrequent Access
#   - One snapshot per month retained indefinitely
#
# Install:
#   sudo cp r2-snapshot.sh /usr/local/bin/
#   sudo chmod +x /usr/local/bin/r2-snapshot.sh
#   # Configure rclone once (interactive):
#   rclone config create quidnug-r2 s3 \
#       provider=Cloudflare \
#       access_key_id=<R2_ACCESS_KEY_ID> \
#       secret_access_key=<R2_SECRET_ACCESS_KEY> \
#       endpoint=https://<ACCOUNT_ID>.r2.cloudflarestorage.com \
#       acl=private
#
# Cron (nightly at 03:30 local):
#   30 3 * * * quidnug /usr/local/bin/r2-snapshot.sh node1 \
#       >>/var/log/quidnug-backup.log 2>&1
#
# Monitoring:
#   On failure the script exits non-zero — wire cron to mail on
#   failure (MAILTO=you@domain), OR add a "healthchecks.io" ping
#   URL via the BACKUP_PING env var and the script will hit it
#   on success + failure for dead-man's-switch alerting.

set -euo pipefail

NODE_NAME="${1:?usage: r2-snapshot.sh <node-name>}"
DATA_DIR="${DATA_DIR:-/var/lib/quidnug/data}"
TMP_DIR="${TMP_DIR:-/var/tmp}"
R2_REMOTE="${R2_REMOTE:-quidnug-r2}"
R2_BUCKET="${R2_BUCKET:-quidnug-node-snapshots}"
NODE_URL="${NODE_URL:-http://127.0.0.1:8087}"
BACKUP_PING="${BACKUP_PING:-}"

ping_hc() {
    [[ -n "${BACKUP_PING}" ]] || return 0
    curl -fsS --retry 3 -m 10 \
        "${BACKUP_PING}${1:+/$1}" -o /dev/null || true
}

fail() {
    echo "[$(date -Is)] FAIL: $*" >&2
    ping_hc "fail"
    exit 1
}

ok() {
    echo "[$(date -Is)] $*"
}

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
SNAPSHOT_NAME="${NODE_NAME}-${STAMP}.tar.zst"
LOCAL_PATH="${TMP_DIR}/${SNAPSHOT_NAME}"

ping_hc "start"
ok "starting snapshot ${SNAPSHOT_NAME}"

# ─── 1. Quiesce ─────────────────────────────────────────────────
# If your version of the CLI doesn't support `snapshot begin`, fall
# back to a short fsync via a health-check + 2s sleep. It's not
# cryptographically guaranteed to be consistent but at the data
# layout is append-only, so the worst outcome is the tail block
# being partially written — the node recovers on restart.
if command -v quidnug-cli >/dev/null 2>&1 && \
   quidnug-cli --help 2>&1 | grep -q "snapshot"; then
    SNAP_REF=$(quidnug-cli snapshot begin \
        --node "${NODE_URL}" --name "${SNAPSHOT_NAME}") \
        || fail "snapshot begin"
    ok "snapshot ref = ${SNAP_REF}"
    trap 'quidnug-cli snapshot end --node "'"${NODE_URL}"'" \
          --ref "'"${SNAP_REF}"'" || true' EXIT
else
    ok "CLI has no snapshot command; relying on append-only layout"
    curl -fsS "${NODE_URL}/api/health" >/dev/null || fail "node unreachable"
    sleep 2
fi

# ─── 2. Archive ─────────────────────────────────────────────────
# -C flags tar into the data dir so paths in the archive are
# relative to data_dir root; that makes restore trivial.
if ! tar -C "${DATA_DIR}" -cf - . | zstd -19 -T0 -o "${LOCAL_PATH}"; then
    fail "tar/zstd failed"
fi
LOCAL_SIZE=$(stat -c%s "${LOCAL_PATH}")
ok "archive created: ${LOCAL_SIZE} bytes"

# ─── 3. Upload ──────────────────────────────────────────────────
if ! rclone copy "${LOCAL_PATH}" \
    "${R2_REMOTE}:${R2_BUCKET}/${NODE_NAME}/" \
    --s3-no-check-bucket \
    --transfers 1 --checkers 2 \
    --retries 3 --low-level-retries 10; then
    fail "rclone upload"
fi

# ─── 4. Verify ──────────────────────────────────────────────────
REMOTE_SIZE=$(rclone size \
    "${R2_REMOTE}:${R2_BUCKET}/${NODE_NAME}/${SNAPSHOT_NAME}" \
    --json 2>/dev/null | jq -r '.bytes')
if [[ "${REMOTE_SIZE}" != "${LOCAL_SIZE}" ]]; then
    fail "size mismatch: local=${LOCAL_SIZE} remote=${REMOTE_SIZE}"
fi
ok "verified upload size matches (${REMOTE_SIZE} bytes)"

# ─── 5. Cleanup ─────────────────────────────────────────────────
rm -f "${LOCAL_PATH}"
ok "local tempfile removed"

ping_hc  # no suffix = success
ok "snapshot ${SNAPSHOT_NAME} complete"
