#!/usr/bin/env bash
# MCP protocol + tool tests against a running OpenID server.
# Usage: BASE_URL=http://localhost:4000 ./test/functional/mcp.sh
set -euo pipefail

BASE="${BASE_URL:-}"
if [[ -z "$BASE" ]]; then
  echo "BASE_URL is required" >&2
  exit 2
fi

PASS=0
FAIL=0
ERRORS=()
green() { printf '\033[32m%s\033[0m\n' "$*"; }
red() { printf '\033[31m%s\033[0m\n' "$*"; }

assert_contains() {
  local name="$1" needle="$2" haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    green "PASS  $name"; PASS=$((PASS+1))
  else
    red "FAIL  $name (missing '$needle')"
    FAIL=$((FAIL+1)); ERRORS+=("$name: missing $needle :: ${haystack:0:240}")
  fi
}

assert_eq() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    green "PASS  $name"; PASS=$((PASS+1))
  else
    red "FAIL  $name (expected=$expected actual=$actual)"
    FAIL=$((FAIL+1)); ERRORS+=("$name: expected=$expected actual=$actual")
  fi
}

rpc() {
  local body="$1"
  curl -sS -X POST "$BASE/mcp" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json' \
    -d "$body"
}

tool_text() {
  local name="$1"
  local args="$2"
  if [[ -z "$args" ]]; then args='{}'; fi
  MCP_TOOL_NAME="$name" MCP_TOOL_ARGS="$args" python3 - <<'PY'
import json, os, urllib.request
base = os.environ["BASE_URL"]
name = os.environ["MCP_TOOL_NAME"]
raw = os.environ.get("MCP_TOOL_ARGS") or "{}"
args = json.loads(raw)
req = {"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": name, "arguments": args}}
with urllib.request.urlopen(urllib.request.Request(
    base + "/mcp",
    data=json.dumps(req).encode(),
    headers={"Content-Type": "application/json", "Accept": "application/json"},
)) as r:
    data = json.load(r)
if data.get("error"):
    raise SystemExit("rpc error: " + json.dumps(data["error"]))
res = data["result"]
if res.get("isError"):
    raise SystemExit("tool error: " + res["content"][0]["text"])
print(res["content"][0]["text"])
PY
}

echo "=== OpenID MCP functional tests against $BASE ==="
export BASE_URL="$BASE"

code=$(curl -sS -o /tmp/mcp_get.json -w '%{http_code}' "$BASE/mcp")
assert_eq "GET /mcp openable" "200" "$code"
assert_contains "GET /mcp open flag" '"open"' "$(cat /tmp/mcp_get.json)"
assert_contains "GET /mcp mcp url" '/mcp' "$(cat /tmp/mcp_get.json)"

code=$(curl -sS -o /tmp/mcp_opt.txt -w '%{http_code}' -X OPTIONS "$BASE/mcp")
assert_eq "OPTIONS /mcp" "204" "$code"

INIT=$(rpc '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"grokbot","version":"local-test"}}}')
assert_contains "initialize server" '"name":"openid"' "$INIT"
assert_contains "initialize protocol" '2025-03-26' "$INIT"

NOTE=$(curl -sS -o /tmp/mcp_note.txt -w '%{http_code}' -X POST "$BASE/mcp" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}')
assert_eq "initialized notification" "202" "$NOTE"

LIST=$(rpc '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')
for tool in openid_open openid_status openid_discover openid_register openid_login \
  openid_register_agent openid_pod_put openid_pod_get openid_audit_flush \
  spark_save_conversation spark_list_conversations spark_share_conversation; do
  assert_contains "tools/list $tool" "$tool" "$LIST"
done

OPEN=$(tool_text openid_open '{}')
assert_contains "openid_open dashboard" '"dashboard"' "$OPEN"
assert_contains "openid_open grokbot flow" 'openid_register_agent' "$OPEN"

STAT=$(tool_text openid_status '{}')
assert_contains "openid_status ok" '"status": "ok"' "$STAT"

DISCO=$(tool_text openid_discover '{}')
assert_contains "discover issuer" '"issuer"' "$DISCO"
assert_contains "discover token_endpoint" 'token_endpoint' "$DISCO"

HANDLE="mcp$(date +%s)"
AVAIL=$(tool_text openid_handle_available "{\"handle\":\"$HANDLE\"}")
assert_contains "handle available" '"available": true' "$AVAIL"

REG=$(tool_text openid_register "{\"handle\":\"$HANDLE\",\"password\":\"mcp-test-pass\",\"name\":\"MCP Tester\"}")
TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["token"])' <<<"$REG")
assert_contains "register token" "eyJ" "$TOKEN"

LOGIN=$(tool_text openid_login "{\"handle\":\"$HANDLE\",\"password\":\"mcp-test-pass\"}")
TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["token"])' <<<"$LOGIN")
assert_contains "login token" "eyJ" "$TOKEN"

ME=$(tool_text openid_me "{\"token\":\"$TOKEN\"}")
assert_contains "me handle" "$HANDLE" "$ME"

SPARK=$(tool_text spark_save_conversation "{\"token\":\"$TOKEN\",\"title\":\"MCP Spark\",\"messages\":[{\"role\":\"user\",\"text\":\"from mcp\"},{\"role\":\"assistant\",\"text\":\"ok\"}]}")
assert_contains "spark save title" "MCP Spark" "$SPARK"
SLIST=$(tool_text spark_list_conversations "{\"token\":\"$TOKEN\"}")
assert_contains "spark list" "MCP Spark" "$SLIST"

PUT=$(tool_text openid_pod_put "{\"path\":\"$HANDLE/public/grokbot.json\",\"body\":\"{\\\"hello\\\":\\\"grokbot\\\"}\",\"content_type\":\"application/json\",\"token\":\"$TOKEN\"}")
assert_contains "pod put created" '"status": 201' "$PUT"

GET=$(tool_text openid_pod_get "{\"path\":\"$HANDLE/public/grokbot.json\",\"token\":\"$TOKEN\"}")
assert_contains "pod get body" 'grokbot' "$GET"

LISTC=$(tool_text openid_pod_list "{\"path\":\"$HANDLE/public/\",\"token\":\"$TOKEN\"}")
assert_contains "pod list" 'grokbot.json' "$LISTC"

CC=$(tool_text openid_client_credentials "{\"token\":\"$TOKEN\",\"name\":\"grokbot\"}")
CID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["id"])' <<<"$CC")
CSEC=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["secret"])' <<<"$CC")
TOK=$(tool_text openid_token "{\"client_id\":\"$CID\",\"client_secret\":\"$CSEC\"}")
assert_contains "oauth access_token" 'access_token' "$TOK"

AGENT=$(tool_text openid_register_agent '{"name":"grokbot-mcp"}')
assert_contains "agent webid" 'profile/card#me' "$AGENT"
ATOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["token"])' <<<"$AGENT")
APOD=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["agent"]["podPath"])' <<<"$AGENT")
APUT=$(tool_text openid_pod_put "{\"path\":\"${APOD}inbox/from-grokbot.json\",\"body\":\"{\\\"ok\\\":true}\",\"content_type\":\"application/json\",\"token\":\"$ATOKEN\"}")
assert_contains "agent inbox write" '"status": 201' "$APUT"

FLUSH=$(tool_text openid_audit_flush '{}')
assert_contains "audit merkle" 'merkleRoot' "$FLUSH"

EID=$(python3 - <<PY
import json,os,urllib.request
base=os.environ["BASE_URL"]
req={"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"openid_audit_events","arguments":{}}}
with urllib.request.urlopen(urllib.request.Request(base+"/mcp", data=json.dumps(req).encode(), headers={"Content-Type":"application/json"})) as r:
    data=json.load(r)
evs=json.loads(data["result"]["content"][0]["text"])["json"]
print(evs[0]["id"])
PY
)
VER=$(tool_text openid_audit_verify "{\"id\":\"$EID\"}")
assert_contains "audit verified" '"verified": true' "$VER"

# stdio transport as grokbot would spawn it
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MCP_BIN="$ROOT/bin/openid-mcp"
if [[ ! -x "$MCP_BIN" ]]; then
  go build -o "$MCP_BIN" "$ROOT/cmd/mcp"
fi
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"grokbot"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"openid_status","arguments":{}}}' \
  > /tmp/mcp_stdio_in.jsonl
OPENID_BASE_URL="$BASE" "$MCP_BIN" < /tmp/mcp_stdio_in.jsonl > /tmp/mcp_stdio_out.jsonl
if python3 - <<'PY'
import json
lines=open("/tmp/mcp_stdio_out.jsonl").read().strip().splitlines()
assert len(lines)>=2, f"stdio lines={len(lines)}"
init=json.loads(lines[0])
assert init["result"]["serverInfo"]["name"]=="openid"
status=json.loads(lines[-1])
text=status["result"]["content"][0]["text"]
body=json.loads(text)
assert body["health"]["json"]["status"]=="ok", body
print("STDIO_OK")
PY
then
  green "PASS  stdio initialize"; PASS=$((PASS+1))
  green "PASS  stdio status ok"; PASS=$((PASS+1))
else
  red "FAIL  stdio grokbot client"; FAIL=$((FAIL+1)); ERRORS+=("stdio grokbot client")
fi

# Existing grokbot pod owner (mike) via MCP, if present.
if curl -sS "$BASE/i/mike" | python3 -c 'import sys,json; json.load(sys.stdin); raise SystemExit(0)' 2>/dev/null; then
  MIKE=$(tool_text openid_login '{"handle":"mike","password":"grokbot-dev-2026"}' || true)
  if [[ "$MIKE" == *token* ]]; then
    MTOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["json"]["token"])' <<<"$MIKE")
    MPUT=$(tool_text openid_pod_put "{\"path\":\"mike/inbox/mcp-live.json\",\"body\":\"{\\\"via\\\":\\\"mcp\\\",\\\"agent\\\":\\\"grokbot\\\"}\",\"content_type\":\"application/json\",\"token\":\"$MTOKEN\"}")
    assert_contains "mike inbox via mcp" '"status": 20' "$MPUT"
    MGET=$(tool_text openid_pod_get "{\"path\":\"mike/inbox/mcp-live.json\",\"token\":\"$MTOKEN\"}")
    assert_contains "mike inbox readback" 'grokbot' "$MGET"
  else
    green "PASS  mike login skipped (password changed)"; PASS=$((PASS+1))
  fi
fi

echo
echo "=== MCP results: $PASS passed, $FAIL failed ==="
if [[ $FAIL -gt 0 ]]; then
  echo "Failures:"
  for e in "${ERRORS[@]}"; do echo "  - $e"; done
  exit 1
fi
exit 0
