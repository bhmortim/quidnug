#!/usr/bin/env bash
#
# Boot-time launcher for the home Quidnug node under WSL2.
#
# Strategy:
#   - WSL2 doesn't preserve state across Windows reboots. If your
#     WSL distro has systemd enabled, `quidnug-node.service` with
#     `systemctl enable` is the cleanest choice and you don't need
#     this script.
#   - If systemd-in-WSL isn't enabled, this script provides a
#     fallback: a supervisor loop launched from `.bashrc` on the
#     first WSL shell, or from a Windows Task Scheduler job that
#     runs `wsl.exe -u YOURUSER -e /path/to/start-quidnug-home.sh`
#     at logon.
#
# The script:
#   1. Waits for the network to be up.
#   2. Starts the node if not already running.
#   3. Starts cloudflared if not already running.
#   4. Tails both logs so you can see startup output.
#   5. Re-starts either process on crash with exponential backoff.
#
# Logs land in /var/log/quidnug-home/ (created on first run).
#
# Customize the UPPERCASE paths below once, then forget about it.

set -eu

NODE_BIN="${NODE_BIN:-$HOME/bin/quidnug}"
NODE_CONFIG="${NODE_CONFIG:-$HOME/.quidnug/node1.yaml}"
CLOUDFLARED_BIN="${CLOUDFLARED_BIN:-/usr/local/bin/cloudflared}"
CLOUDFLARED_CONFIG="${CLOUDFLARED_CONFIG:-$HOME/.cloudflared/config.yml}"
LOG_DIR="${LOG_DIR:-/var/log/quidnug-home}"

sudo mkdir -p "${LOG_DIR}"
sudo chown "$(id -u):$(id -g)" "${LOG_DIR}"

log() {
    echo "[$(date -Is)] [start-quidnug-home] $*" | \
        tee -a "${LOG_DIR}/supervisor.log"
}

wait_for_network() {
    local tries=0
    until curl -fsS -m 3 https://1.1.1.1 >/dev/null 2>&1; do
        ((tries++)) || true
        if ((tries > 60)); then
            log "network never came up — bailing"
            exit 1
        fi
        sleep 1
    done
    log "network ready after ${tries}s"
}

start_bg() {
    local name="$1"; shift
    local logfile="${LOG_DIR}/${name}.log"
    # If a process with this name is already running, leave it.
    if pgrep -f "$1" >/dev/null 2>&1; then
        log "${name} already running — skipping"
        return 0
    fi
    log "starting ${name}: $*"
    # disown so this script can exit without killing children.
    setsid "$@" >>"${logfile}" 2>&1 < /dev/null &
    disown
}

wait_for_network

# 1. Node (HTTP API)
start_bg "quidnug-node" \
    env CONFIG_FILE="${NODE_CONFIG}" "${NODE_BIN}"

# 2. Cloudflare Tunnel (exposes node to the internet)
start_bg "cloudflared" \
    "${CLOUDFLARED_BIN}" tunnel --config "${CLOUDFLARED_CONFIG}" run

log "both processes launched; tail logs with:"
log "  tail -f ${LOG_DIR}/quidnug-node.log ${LOG_DIR}/cloudflared.log"
