#!/usr/bin/env bash
# Prove: spark_login(handle,pass) → token → spark_save_conversation →
# spark_share_conversation → public GET 200.
# Usage: MCP_URL=https://<vercel-preview> ./scripts/prove-spark-login.sh
set -euo pipefail

MCP="${MCP_URL:-${1:-}}"
if [[ -z "$MCP" ]]; then
  echo "MCP_URL is required (Vercel preview origin or /mcp URL)" >&2
  exit 2
fi
MCP="${MCP%/}"
if [[ "$MCP" != */mcp ]]; then
  MCP="$MCP/mcp"
fi

HANDLE="${SPARK_HANDLE:-prove$(date +%s)}"
PASS="${SPARK_PASSWORD:-prove-pass-12345}"
NAME="${SPARK_NAME:-Prove Login}"

echo "=== spark_login proof against $MCP ==="

LIST=$(curl -sS -X POST "$MCP" \
  -H 'Content-Type: application/json' -H 'Accept: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
echo "$LIST" | python3 -c 'import json,sys; names=[t["name"] for t in json.load(sys.stdin)["result"]["tools"]];
assert "spark_login" in names, names
assert "spark_save_conversation" in names
assert "spark_share_conversation" in names
print("tools/list ok:", ", ".join(names))'

export MCP HANDLE PASS NAME
python3 <<'PY'
import json, os, urllib.request, ssl

mcp = os.environ["MCP"]
handle = os.environ["HANDLE"]
password = os.environ["PASS"]
name = os.environ["NAME"]

def call(tool_name, args):
    req = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {"name": tool_name, "arguments": args},
    }
    with urllib.request.urlopen(urllib.request.Request(
        mcp,
        data=json.dumps(req).encode(),
        headers={"Content-Type": "application/json", "Accept": "application/json"},
    ), timeout=90) as r:
        data = json.load(r)
    if data.get("error"):
        raise SystemExit("rpc error: " + json.dumps(data["error"]))
    res = data["result"]
    text = (res.get("content") or [{}])[0].get("text") or ""
    if res.get("isError"):
        raise SystemExit("tool error " + tool_name + ": " + text)
    return json.loads(text)

print("register handle=" + handle)
reg = call("spark_register", {"handle": handle, "password": password, "name": name})
dump = json.dumps(reg)
assert reg.get("ok") and reg.get("token"), reg
assert password not in dump, "password echoed from spark_register"
print("spark_register ok handle=", reg.get("handle"), "expires=", reg.get("expires"))

login = call("spark_login", {"handle": handle, "password": password})
dump = json.dumps(login)
assert login.get("ok") and login.get("token") and login.get("tokenType") == "Bearer", login
assert password not in dump, "password echoed from spark_login"
token = login["token"]
print("spark_login ok webId=", login.get("webId"))

saved = call("spark_save_conversation", {
    "token": token,
    "title": "Prove spark_login",
    "messages": [
        {"role": "user", "text": "save this conversation from spark_login proof", "timestamp": "2026-09-01T22:00:00Z"},
        {"role": "assistant", "text": "saved to your Solid pod", "timestamp": "2026-09-01T22:00:05Z"},
    ],
})
cid = saved.get("id")
assert cid, saved
print("saved id=" + cid)

shared = call("spark_share_conversation", {"id": cid, "token": token})
share_url = shared.get("shareUrl") or (shared.get("share") or {}).get("url") or ""
assert share_url, shared
print("shareUrl=" + share_url)

req = urllib.request.Request(share_url, headers={"Accept": "application/json"})
with urllib.request.urlopen(req, timeout=60) as r:
    code = r.status
    body = r.read()
print("public GET", share_url, "->", code)
assert code == 200, code
doc = json.loads(body)
blob = json.dumps(doc)
assert "Prove" in blob or "spark_login" in blob, doc
print("public body ok title=", doc.get("title"))
print("=== proof passed ===")
print("handle=" + handle)
print("share=" + share_url)
PY
