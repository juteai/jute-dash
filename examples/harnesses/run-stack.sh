#!/usr/bin/env bash
set -euo pipefail

: "${STACK_NAME:?STACK_NAME is required}"
: "${ROOT_DIR:?ROOT_DIR is required}"
: "${WEB_DIR:?WEB_DIR is required}"
: "${FIXTURE_DIR:?FIXTURE_DIR is required}"
: "${CONFIG:?CONFIG is required}"
: "${DATA_DIR:?DATA_DIR is required}"
: "${AGENT_TARGET:?AGENT_TARGET is required}"

HUB_URL="${HUB_URL:-http://127.0.0.1:8787}"
WEB_URL="${WEB_URL:-http://127.0.0.1:5173}"
MCP_URL="${MCP_URL:-http://127.0.0.1:8790/mcp}"
JUTE_MCP_AGENT_ID="${JUTE_MCP_AGENT_ID:-}"
AGENT_CARD_URL="${AGENT_CARD_URL:-http://127.0.0.1:9797/.well-known/agent-card.json}"
AGENT_RPC_URL="${AGENT_RPC_URL:-http://127.0.0.1:9797/invoke}"
MCP_ENABLED="${MCP_ENABLED:-false}"
NPM="${NPM:-npm}"

HUB_PID=""
AGENT_PID=""
WEB_PID=""

cleanup() {
	if [[ -n "${WEB_PID}" ]]; then kill "${WEB_PID}" 2>/dev/null || true; fi
	if [[ -n "${AGENT_PID}" ]]; then kill "${AGENT_PID}" 2>/dev/null || true; fi
	if [[ -n "${HUB_PID}" ]]; then kill "${HUB_PID}" 2>/dev/null || true; fi
}
trap cleanup INT TERM EXIT

need_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Missing required command: $1" >&2
		exit 1
	fi
}

url_host_port() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import urlparse

parsed = urlparse(sys.argv[1])
if not parsed.hostname:
    raise SystemExit(1)
port = parsed.port
if port is None:
    port = 443 if parsed.scheme == "https" else 80
print(f"{parsed.hostname} {port}")
PY
}

assert_port_free() {
	local label="$1"
	local url="$2"
	local host port
	read -r host port < <(url_host_port "$url")
	if ! python3 - "$host" "$port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
probe_host = "127.0.0.1" if host in {"0.0.0.0", "::"} else host
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.settimeout(0.5)
try:
    sock.bind((probe_host, port))
except OSError:
    raise SystemExit(1)
finally:
    sock.close()
PY
	then
		echo "$label port appears to be in use: $host:$port" >&2
		echo "Stop the existing process or override the URL/port before running this harness." >&2
		exit 1
	fi
}

wait_http_get() {
	local label="$1"
	local url="$2"
	local pid="${3:-}"
	local timeout="${4:-120}"
	local elapsed=0
	while (( elapsed < timeout )); do
		if curl -fsS "$url" >/dev/null 2>&1; then
			return 0
		fi
		if [[ -n "$pid" ]] && ! kill -0 "$pid" 2>/dev/null; then
			echo "$label exited before it became ready." >&2
			wait "$pid" || true
			exit 1
		fi
		sleep 1
		elapsed=$((elapsed + 1))
	done
	echo "Timed out waiting for $label at $url." >&2
	exit 1
}

wait_mcp() {
	local timeout="${1:-120}"
	local elapsed=0
	while (( elapsed < timeout )); do
		local headers=(-H 'Content-Type: application/json')
		if [[ -n "$JUTE_MCP_AGENT_ID" ]]; then
			headers+=(-H "X-Jute-Agent-ID: $JUTE_MCP_AGENT_ID")
		fi
		if curl -fsS "$MCP_URL" "${headers[@]}" \
			-d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' 2>/dev/null | grep -q 'jute_skill_read_context'; then
			return 0
		fi
		if [[ -n "$HUB_PID" ]] && ! kill -0 "$HUB_PID" 2>/dev/null; then
			echo "Jute hub exited before MCP became ready." >&2
			wait "$HUB_PID" || true
			exit 1
		fi
		sleep 1
		elapsed=$((elapsed + 1))
	done
	echo "Timed out waiting for Jute MCP at $MCP_URL." >&2
	exit 1
}

need_command go
need_command curl
need_command python3
need_command "$NPM"

if [[ ! -f "$CONFIG" ]]; then
	echo "Harness config not found: $CONFIG" >&2
	exit 1
fi

echo "$STACK_NAME"
echo "  Config:         $CONFIG"
echo "  Data dir:       $DATA_DIR"
echo "  Jute hub:       $HUB_URL"
if [[ "$MCP_ENABLED" == "true" ]]; then
	echo "  Jute MCP:       $MCP_URL"
	if [[ -n "$JUTE_MCP_AGENT_ID" ]]; then echo "  MCP agent ID:   $JUTE_MCP_AGENT_ID"; fi
fi
echo "  Fixture Card:   $AGENT_CARD_URL"
echo "  Fixture RPC:    $AGENT_RPC_URL"
echo "  Jute web:       $WEB_URL"

assert_port_free "Jute hub" "$HUB_URL"
assert_port_free "A2A fixture" "$AGENT_RPC_URL"
assert_port_free "Jute web" "$WEB_URL"
if [[ "$MCP_ENABLED" == "true" ]]; then assert_port_free "Jute MCP" "$MCP_URL"; fi

echo "Ensuring web and fixture dependencies."
if [[ ! -d "$WEB_DIR/node_modules" ]]; then
	(cd "$WEB_DIR" && "$NPM" install)
fi
(cd "$FIXTURE_DIR" && go mod download)

echo "Resetting harness store: $DATA_DIR"
rm -rf "$DATA_DIR"

echo "Starting Jute hub."
(cd "$ROOT_DIR/apps/hub" && go run ./cmd/juted -config "$CONFIG" -data-dir "$DATA_DIR") & HUB_PID=$!
wait_http_get "Jute hub" "$HUB_URL/healthz" "$HUB_PID" 120

if [[ "$MCP_ENABLED" == "true" ]]; then
	echo "Waiting for Jute MCP."
	wait_mcp 120
fi

echo "Starting A2A fixture."
(cd "$FIXTURE_DIR" && make "$AGENT_TARGET" JUTE_MCP_URL="$MCP_URL" JUTE_MCP_AGENT_ID="$JUTE_MCP_AGENT_ID") & AGENT_PID=$!
wait_http_get "A2A fixture" "$AGENT_CARD_URL" "$AGENT_PID" 900

echo "Starting web UI."
(cd "$WEB_DIR" && "$NPM" run dev -- --strictPort) & WEB_PID=$!
wait_http_get "Jute web" "$WEB_URL" "$WEB_PID" 120

echo "Full stack is ready. Press Ctrl-C to stop."
wait "$HUB_PID" "$AGENT_PID" "$WEB_PID"
