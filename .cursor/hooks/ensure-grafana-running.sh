#!/usr/bin/env bash
# Keep Grafana available for demo sessions without waiting on Air rebuilds.
# Prefer the prebuilt binary + public/build assets. Rebuild only when needed.
#
# Persistence strategy:
# - Prefer a user LaunchAgent so Grafana survives hook/shell exits
# - Fall back to a double-fork daemon if launchctl is unavailable

set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LOG_DIR="$ROOT/.cursor/logs"
PID_FILE="$LOG_DIR/grafana.pid"
LOG_FILE="$LOG_DIR/grafana-server.log"
BIN="$ROOT/bin/grafana"
PORT=3000
URL="http://127.0.0.1:${PORT}/"
LABEL="com.cursor.grafana-demo"
PLIST_SRC="$ROOT/.cursor/launchd/${LABEL}.plist"
PLIST_DST="${HOME}/Library/LaunchAgents/${LABEL}.plist"

mkdir -p "$LOG_DIR"

is_up() {
  curl -s -o /dev/null -w "%{http_code}" --max-time 2 "$URL" | grep -Eq '^(200|302|301)$'
}

status_json() {
  local state="$1"
  local detail="$2"
  python3 -c 'import json,sys; print(json.dumps({"additional_context": sys.argv[1]}))' \
    "Grafana local demo server: ${state}. ${detail} URL: ${URL} Login: admin/admin. Prefer .cursor/hooks/ensure-grafana-running.sh (or the start-dev-server skill fast path) over make run unless you need Go hot-reload."
}

wait_until_up() {
  local seconds="${1:-20}"
  local i=0
  while (( i < seconds * 2 )); do
    if is_up; then
      return 0
    fi
    sleep 0.5
    i=$((i + 1))
  done
  return 1
}

start_via_launchagent() {
  mkdir -p "${HOME}/Library/LaunchAgents" "$(dirname "$PLIST_SRC")"

  # Render plist with absolute paths for this machine/workspace.
  cat >"$PLIST_SRC" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BIN}</string>
    <string>server</string>
    <string>--homepath=${ROOT}</string>
    <string>--packaging=dev</string>
    <string>cfg:app_mode=development</string>
  </array>
  <key>WorkingDirectory</key>
  <string>${ROOT}</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${LOG_FILE}</string>
  <key>StandardErrorPath</key>
  <string>${LOG_FILE}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>GF_DEFAULT_APP_MODE</key>
    <string>development</string>
  </dict>
</dict>
</plist>
EOF

  cp "$PLIST_SRC" "$PLIST_DST"

  # Reload idempotently.
  launchctl bootout "gui/$(id -u)/${LABEL}" >/dev/null 2>&1 || true
  launchctl bootstrap "gui/$(id -u)" "$PLIST_DST"
  launchctl enable "gui/$(id -u)/${LABEL}" >/dev/null 2>&1 || true
  launchctl kickstart -k "gui/$(id -u)/${LABEL}" >/dev/null 2>&1 || true
}

start_via_double_fork() {
  cd "$ROOT" || return 1
  # Fully detach from the parent session so Cursor hook shells don't kill Grafana.
  /usr/bin/python3 - <<'PY'
import os, subprocess, pathlib
root = pathlib.Path(os.environ["ROOT"])
bin_path = root / "bin" / "grafana"
log_file = root / ".cursor" / "logs" / "grafana-server.log"
pid_file = root / ".cursor" / "logs" / "grafana.pid"
log_file.parent.mkdir(parents=True, exist_ok=True)
log_f = open(log_file, "a", buffering=1)
proc = subprocess.Popen(
    [str(bin_path), "server", f"--homepath={root}", "--packaging=dev", "cfg:app_mode=development"],
    cwd=str(root),
    stdout=log_f,
    stderr=log_f,
    start_new_session=True,
)
pid_file.write_text(str(proc.pid))
print(proc.pid)
PY
}

if is_up; then
  status_json "already running" "Port ${PORT} is healthy."
  exit 0
fi

if [[ ! -x "$BIN" ]]; then
  status_json "binary missing" "Expected ${BIN}. Run a backend build once, then re-open the workspace."
  exit 0
fi

export ROOT

if command -v launchctl >/dev/null 2>&1; then
  if start_via_launchagent; then
    if wait_until_up 25; then
      status_json "started" "Running via LaunchAgent ${LABEL} (KeepAlive enabled)."
      exit 0
    fi
  fi
fi

pid="$(start_via_double_fork || true)"
if [[ -n "${pid:-}" ]] && wait_until_up 25; then
  status_json "started" "Started detached process (pid ${pid})."
  exit 0
fi

status_json "starting" "Launch attempted; still warming up or failed. Check ${LOG_FILE}."
exit 0
