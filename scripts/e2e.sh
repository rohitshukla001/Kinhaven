#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

GO=${GO:-go}
MCP_PORT=${E2E_MCP_PORT:-18080}
SIM_PORT=${E2E_SIM_PORT:-18090}
WORK=$(mktemp -d)
PIDS=()

cleanup() {
	for pid in ${PIDS[@]+"${PIDS[@]}"}; do
		kill "$pid" 2>/dev/null || true
		wait "$pid" 2>/dev/null || true
	done
	rm -rf "$WORK"
}
trap cleanup EXIT

fail() {
	echo "FAIL: $1" >&2
	for log in "$WORK"/*.log; do
		[ -f "$log" ] && { echo "--- $(basename "$log")" >&2; cat "$log" >&2; }
	done
	exit 1
}

pass() {
	echo "ok    $1"
}

expect() {
	grep -qF -- "$2" <<<"$3" || fail "$1: output does not contain \"$2\""
}

wait_for() {
	for _ in $(seq 1 50); do
		curl -fsS -o /dev/null "$1" 2>/dev/null && return 0
		sleep 0.2
	done
	fail "$1 did not answer"
}

export KINHAVEN_DATA_DIR="$WORK/data"
export KINHAVEN_TIMEZONE=${KINHAVEN_TIMEZONE:-Asia/Kolkata}
export KINHAVEN_ADDR="127.0.0.1:$MCP_PORT"
export KINHAVEN_ALLOWED_ORIGINS="http://127.0.0.1:$SIM_PORT"
export KINHAVEN_ALERTS=log
export KINHAVEN_ALERT_INTERVAL=1s
export KINHAVEN_SIM_ADDR="127.0.0.1:$SIM_PORT"
export KINHAVEN_SIM_MCP_URL="http://127.0.0.1:$MCP_PORT/mcp"
export KINHAVEN_SIM_MODEL=mock

$GO build -o "$WORK/bin/" ./cmd/...
pass "build"

out=$("$WORK/bin/kinhaven-seed")
expect "seed" "Seeded Kamala ($KINHAVEN_TIMEZONE)" "$out"
pass "seed"

"$WORK/bin/kinhaven-server" >"$WORK/server.log" 2>&1 &
PIDS+=($!)
wait_for "http://127.0.0.1:$MCP_PORT/healthz"
pass "server health"

init='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"e2e","version":"1"}}}'
out=$(curl -sS -X POST "http://127.0.0.1:$MCP_PORT/mcp" \
	-H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d "$init")
expect "mcp initialize" '"name":"kinhaven"' "$out"
pass "mcp initialize"

code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:$MCP_PORT/mcp" \
	-H 'Origin: http://attacker.example' -H 'Content-Type: application/json' \
	-H 'Accept: application/json, text/event-stream' -d "$init")
[ "$code" = 403 ] || fail "foreign origin: got HTTP $code, want 403"
pass "foreign origin is rejected"

out=$(printf 'What is due today?\nHow was this week?\nI took my Metformin\nI feel good today\n\n' | "$WORK/bin/kinhaven-sim" console)
expect "console" "with 10 tools, model mock" "$out"
expect "console" "[get_schedule]" "$out"
expect "console" "Kamala" "$out"
expect "console" "[get_adherence_summary]" "$out"
expect "console" "[log_dose]" "$out"
expect "console" "[record_checkin]" "$out"
pass "console conversation"

"$WORK/bin/kinhaven-sim" >"$WORK/sim.log" 2>&1 &
PIDS+=($!)
wait_for "http://127.0.0.1:$SIM_PORT/api/info"
out=$(curl -sS -X POST "http://127.0.0.1:$SIM_PORT/api/ask" \
	-H "Origin: http://127.0.0.1:$SIM_PORT" -H 'Content-Type: application/json' \
	-d '{"text":"What is due today?"}')
expect "web ask" '"tool":"get_schedule"' "$out"
expect "web ask" '"resource_uri":"ui://kinhaven/card.html"' "$out"
pass "web simulator ask"

out=$(curl -sS "http://127.0.0.1:$SIM_PORT/api/resource?uri=ui://kinhaven/card.html")
expect "card resource" "ui/initialize" "$out"
pass "MCP App card resource"

if [ $((10#$(TZ=$KINHAVEN_TIMEZONE date +%H))) -ge 8 ]; then
	for _ in $(seq 1 25); do
		grep -q "missed-dose alert" "$WORK/server.log" && break
		sleep 0.2
	done
	expect "missed-dose alert" "Kamala has not taken" "$(cat "$WORK/server.log")"
	pass "missed-dose alert"
fi

echo "PASS"
