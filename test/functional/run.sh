#!/usr/bin/env bash
# Functional tests against a running OpenID server.
# Usage: BASE_URL=http://localhost:3460 ./test/functional/run.sh
set -euo pipefail

BASE="${BASE_URL:-}"
if [[ -z "$BASE" ]]; then
  echo "BASE_URL is required, e.g. BASE_URL=http://localhost:3460 $0" >&2
  exit 2
fi

PASS=0
FAIL=0
ERRORS=()

green() { printf '\033[32m%s\033[0m\n' "$*"; }
red() { printf '\033[31m%s\033[0m\n' "$*"; }

assert_eq() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    green "PASS  $name"; PASS=$((PASS+1))
  else
    red "FAIL  $name (expected=$expected actual=$actual)"
    FAIL=$((FAIL+1)); ERRORS+=("$name: expected=$expected actual=$actual")
  fi
}

assert_contains() {
  local name="$1" needle="$2" haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    green "PASS  $name"; PASS=$((PASS+1))
  else
    red "FAIL  $name (missing '$needle')"
    FAIL=$((FAIL+1)); ERRORS+=("$name: missing $needle :: ${haystack:0:240}")
  fi
}

assert_http() { assert_eq "$1" "$2" "$3"; }

curl_code() {
  # usage: curl_code <outfile> <curl args...>
  local out="$1"; shift
  curl -sS -o "$out" -w '%{http_code}' "$@"
}

echo "=== OpenID functional tests against $BASE ==="

code=$(curl_code /tmp/ft_body "$BASE/health")
assert_http "health status" "200" "$code"
assert_contains "health body" '"status":"ok"' "$(cat /tmp/ft_body)"

code=$(curl_code /tmp/ft_body -H 'Accept: text/html' "$BASE/")
assert_http "root dashboard" "200" "$code"
assert_contains "root dashboard title" "Solid server" "$(cat /tmp/ft_body)"
code=$(curl_code /tmp/ft_body -L "$BASE/")
assert_http "root status redirect" "200" "$code"
body=$(cat /tmp/ft_body)
if [[ "$body" == *SolidGo* || "$body" == *OpenID* || "$body" == *'"agents"'* ]]; then
  green "PASS  root identity"; PASS=$((PASS+1))
else
  red "FAIL  root identity"; FAIL=$((FAIL+1)); ERRORS+=("root identity: $body")
fi

code=$(curl_code /tmp/ft_body "$BASE/.well-known/openid-configuration")
assert_http "oidc discovery" "200" "$code"
assert_contains "oidc issuer" '"issuer"' "$(cat /tmp/ft_body)"
assert_contains "oidc token_endpoint" 'token_endpoint' "$(cat /tmp/ft_body)"

code=$(curl_code /tmp/ft_body "$BASE/.well-known/solid")
assert_http "solid description" "200" "$code"

REG=$(curl -sS -c /tmp/ft_cookies -X POST "$BASE/idp/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"func@example.com","password":"testpass123","name":"Func User","createPod":true}')
echo "$REG" > /tmp/ft_reg.json
TOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/ft_reg.json"))["token"])')
WEBID=$(python3 -c 'import json; print(json.load(open("/tmp/ft_reg.json"))["webId"])')
POD=$(python3 -c 'import json; print(json.load(open("/tmp/ft_reg.json"))["account"]["podPath"])')
assert_contains "register token" "eyJ" "$TOKEN"
assert_contains "register webid" "profile/card#me" "$WEBID"

code=$(curl_code /tmp/ft_body "$BASE/${POD}profile/card")
assert_http "public profile GET" "200" "$code"
assert_contains "profile foaf name" "Func User" "$(cat /tmp/ft_body)"

LOGIN=$(curl -sS -c /tmp/ft_cookies -X POST "$BASE/idp/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"func@example.com","password":"testpass123"}')
assert_contains "login token" "eyJ" "$(echo "$LOGIN" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("token",""))')"

code=$(curl_code /tmp/ft_body -D /tmp/ft_hdr -X PUT "$BASE/workspace/note.txt" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/plain' --data 'hello functional')
assert_http "authenticated PUT" "201" "$code"
ETAG=$(grep -i '^etag:' /tmp/ft_hdr | awk '{print $2}' | tr -d '\r')
assert_contains "PUT etag" '"' "$ETAG"

code=$(curl_code /tmp/ft_body -D /tmp/ft_hdr "$BASE/workspace/note.txt")
assert_http "public GET resource" "200" "$code"
assert_eq "GET body" "hello functional" "$(cat /tmp/ft_body)"
assert_contains "GET content-type" "text/plain" "$(grep -i '^content-type:' /tmp/ft_hdr | tr -d '\r')"
assert_contains "GET link acl" 'rel="acl"' "$(cat /tmp/ft_hdr)"

code=$(curl_code /tmp/ft_body -I "$BASE/workspace/note.txt")
assert_http "HEAD resource" "200" "$code"

code=$(curl_code /tmp/ft_body "$BASE/workspace/")
assert_http "container listing" "200" "$code"
assert_contains "ldp contains" "ldp#contains" "$(cat /tmp/ft_body)"
assert_contains "contains note" "note.txt" "$(cat /tmp/ft_body)"

code=$(curl_code /tmp/ft_body -H "If-None-Match: $ETAG" "$BASE/workspace/note.txt")
assert_http "If-None-Match 304" "304" "$code"

code=$(curl_code /tmp/ft_body -X PUT "$BASE/workspace/note.txt" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/plain' \
  -H 'If-Match: "deadbeef"' --data 'stale')
assert_http "If-Match precondition fail" "412" "$code"

code=$(curl_code /tmp/ft_body -X PUT "$BASE/workspace/denied.txt" \
  -H 'Content-Type: text/plain' --data 'nope')
assert_http "public PUT denied" "401" "$code"

code=$(curl_code /tmp/ft_body -D /tmp/ft_hdr -X POST "$BASE/workspace/" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/plain' \
  -H 'Slug: posted' --data 'via post')
assert_http "POST create" "201" "$code"
LOC=$(grep -i '^location:' /tmp/ft_hdr | awk '{print $2}' | tr -d '\r')
assert_contains "POST location" "posted" "$LOC"

curl -sS -X PUT "$BASE/workspace/graph.ttl" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/turtle' \
  --data '<http://ex/a> <http://ex/b> <http://ex/c> .' >/dev/null
code=$(curl_code /tmp/ft_body -X PATCH "$BASE/workspace/graph.ttl" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/sparql-update' \
  --data 'INSERT DATA { <http://ex/a> <http://ex/b> <http://ex/d> . }')
assert_http "SPARQL PATCH" "200" "$code"
assert_contains "PATCH inserted triple" "http://ex/d" "$(curl -sS "$BASE/workspace/graph.ttl")"

code=$(curl_code /tmp/ft_body -D /tmp/ft_hdr -X OPTIONS "$BASE/workspace/note.txt")
assert_http "OPTIONS" "200" "$code"
assert_contains "Accept-Patch" "sparql-update" "$(cat /tmp/ft_hdr)"

code=$(curl_code /tmp/ft_body -X DELETE "$BASE/workspace/posted" -H "Authorization: Bearer $TOKEN")
assert_http "DELETE resource" "204" "$code"
code=$(curl_code /tmp/ft_body "$BASE/workspace/posted")
assert_http "GET deleted 404" "404" "$code"

CC=$(curl -sS -b /tmp/ft_cookies -X POST "$BASE/idp/client-credentials" \
  -H 'Content-Type: application/json' -d '{"name":"bot"}')
echo "$CC" > /tmp/ft_cc.json
CID=$(python3 -c 'import json; print(json.load(open("/tmp/ft_cc.json"))["id"])')
CSEC=$(python3 -c 'import json; print(json.load(open("/tmp/ft_cc.json"))["secret"])')
TOK_RESP=$(curl -sS -X POST "$BASE/oauth/token" \
  -d "grant_type=client_credentials&client_id=$CID&client_secret=$CSEC")
CCTOKEN=$(echo "$TOK_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("access_token",""))')
assert_contains "client_credentials token" "eyJ" "$CCTOKEN"
code=$(curl_code /tmp/ft_body -X PUT "$BASE/workspace/from-bot.txt" \
  -H "Authorization: Bearer $CCTOKEN" -H 'Content-Type: text/plain' --data 'bot wrote')
assert_http "client_credentials PUT" "201" "$code"

AGENT=$(curl -sS -X POST "$BASE/agents" -H 'Content-Type: application/json' -d '{"name":"FuncAgent"}')
echo "$AGENT" > /tmp/ft_agent.json
ATOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/ft_agent.json"))["token"])')
APOD=$(python3 -c 'import json; print(json.load(open("/tmp/ft_agent.json"))["agent"]["podPath"])')
AWEB=$(python3 -c 'import json; print(json.load(open("/tmp/ft_agent.json"))["agent"]["webId"])')
APUB=$(python3 -c 'import json; print(json.load(open("/tmp/ft_agent.json"))["agent"]["publicKey"])')
APRIV=$(python3 -c 'import json; print(json.load(open("/tmp/ft_agent.json")).get("privateKey",""))')
assert_contains "agent webid" "#me" "$AWEB"
[[ -n "$APUB" ]] && { green "PASS  agent pubkey present"; PASS=$((PASS+1)); } || { red "FAIL  agent pubkey"; FAIL=$((FAIL+1)); }

code=$(curl_code /tmp/ft_body "$BASE/${APOD}profile/card")
assert_http "agent profile GET" "200" "$code"

code=$(curl_code /tmp/ft_body -X PUT "$BASE/${APOD}inbox/job.json" \
  -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' --data '{"job":1}')
assert_http "agent PUT inbox" "201" "$code"

# Agent Ed25519 signature via Go helper
if [[ -n "$APRIV" ]]; then
  SIG_JSON=$(cd "$(dirname "$0")/../.." && go run ./test/functional/signagent \
    -priv "$APRIV" -pub "$APUB" -webid "$AWEB" \
    -method PUT -path "/${APOD}inbox/signed.txt" 2>/dev/null || true)
  if [[ -n "$SIG_JSON" ]] && echo "$SIG_JSON" | python3 -c 'import sys,json; json.load(sys.stdin)' >/dev/null 2>&1; then
    TS=$(echo "$SIG_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["ts"])')
    SIG=$(echo "$SIG_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["sig"])')
    code=$(curl_code /tmp/ft_body -X PUT "$BASE/${APOD}inbox/signed.txt" \
      -H "X-Agent-WebID: $AWEB" \
      -H "X-Agent-Public-Key: $APUB" \
      -H "X-Agent-Timestamp: $TS" \
      -H "X-Agent-Signature: $SIG" \
      -H 'Content-Type: text/plain' --data 'signed')
    assert_http "agent signature PUT" "201" "$code"
  else
    red "FAIL  agent signature helper ($SIG_JSON)"
    FAIL=$((FAIL+1)); ERRORS+=("agent signature helper failed")
  fi
fi

FLUSH=$(curl -sS -X POST "$BASE/audit/flush")
echo "$FLUSH" > /tmp/ft_flush.json
ROOT=$(python3 -c 'import json; d=json.load(open("/tmp/ft_flush.json")); print((d or {}).get("merkleRoot",""))')
[[ ${#ROOT} -eq 64 ]] && { green "PASS  audit merkle root hex64"; PASS=$((PASS+1)); } || { red "FAIL  audit merkle root ($ROOT)"; FAIL=$((FAIL+1)); }
assert_contains "audit ots present" '"ots"' "$FLUSH"
assert_contains "audit ipfs cid" "bafy" "$FLUSH"

EVENTS=$(curl -sS "$BASE/audit/events/")
EID=$(echo "$EVENTS" | python3 -c 'import sys,json; e=json.load(sys.stdin); print(e[0]["id"] if e else "")')
[[ -n "$EID" ]] && { green "PASS  audit events nonempty"; PASS=$((PASS+1)); } || { red "FAIL  audit events"; FAIL=$((FAIL+1)); }
VERIFY=$(curl -sS "$BASE/audit/events/$EID/verify")
if echo "$VERIFY" | python3 -c 'import sys,json; d=json.load(sys.stdin); raise SystemExit(0 if d.get("verified") else 1)'; then
  green "PASS  audit verified"; PASS=$((PASS+1))
else
  red "FAIL  audit verified ($VERIFY)"; FAIL=$((FAIL+1)); ERRORS+=("audit verified: $VERIFY")
fi

code=$(curl_code /tmp/ft_body "$BASE/notifications/")
assert_http "notifications describe" "200" "$code"

code=$(curl_code /tmp/ft_body -X POST "$BASE/idp/register" \
  -H 'Content-Type: application/json' -d '{"email":"func@example.com","password":"x"}')
assert_http "duplicate register conflict" "409" "$code"

code=$(curl_code /tmp/ft_body -X PUT "$BASE/workspace/subdir/" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Link: <http://www.w3.org/ns/ldp#BasicContainer>; rel="type"')
assert_http "PUT container" "201" "$code"

# WAC-Allow on authenticated GET
code=$(curl_code /tmp/ft_body -D /tmp/ft_hdr "$BASE/workspace/note.txt" -H "Authorization: Bearer $TOKEN")
assert_http "auth GET" "200" "$code"
if grep -qi 'wac-allow' /tmp/ft_hdr; then
  green "PASS  WAC-Allow header"; PASS=$((PASS+1))
else
  red "FAIL  WAC-Allow header"; FAIL=$((FAIL+1)); ERRORS+=("missing WAC-Allow")
fi

# --- Edge cases ---
code=$(curl_code /tmp/ft_body -X POST "$BASE/idp/login" \
  -H 'Content-Type: application/json' -d '{"email":"func@example.com","password":"wrong"}')
assert_http "wrong password 401" "401" "$code"

code=$(curl_code /tmp/ft_body "$BASE/idp/accounts/me" -H "Authorization: Bearer $TOKEN")
assert_http "accounts/me bearer" "200" "$code"
assert_contains "accounts/me email" "func@example.com" "$(cat /tmp/ft_body)"

# --- Identity session: register → login → me → logout ---
IDHANDLE="id$(date +%s)"
IDREG=$(curl -sS -c /tmp/ft_id_cookies -X POST "$BASE/idp/register" \
  -H 'Content-Type: application/json' \
  -d "{\"handle\":\"$IDHANDLE\",\"password\":\"testpass123\",\"name\":\"ID User\",\"createPod\":true}")
echo "$IDREG" > /tmp/ft_idreg.json
IDTOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/ft_idreg.json"))["token"])')
assert_contains "id register token" "eyJ" "$IDTOKEN"
IDLOGIN=$(curl -sS -c /tmp/ft_id_cookies -X POST "$BASE/idp/login" \
  -H 'Content-Type: application/json' \
  -d "{\"handle\":\"$IDHANDLE\",\"password\":\"testpass123\"}")
IDTOKEN=$(echo "$IDLOGIN" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("token",""))')
assert_contains "id login token" "eyJ" "$IDTOKEN"
code=$(curl_code /tmp/ft_body "$BASE/idp/accounts/me" -H "Authorization: Bearer $IDTOKEN")
assert_http "id me after login" "200" "$code"
assert_contains "id me handle" "$IDHANDLE" "$(cat /tmp/ft_body)"
code=$(curl_code /tmp/ft_body -X POST "$BASE/idp/logout" -b /tmp/ft_id_cookies)
assert_http "id logout" "204" "$code"

# --- Spark conversations: save → list → share GET → unshare 404 ---
code=$(curl_code /tmp/ft_body -X POST "$BASE/conversations" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  --data '{"title":"Func Spark","messages":[{"role":"user","text":"hello spark","timestamp":"2026-09-01T20:15:30+10:00"},{"role":"assistant","text":"saved","timestamp":"2026-09-01T10:16:00Z"}]}')
assert_http "conversation save" "201" "$code"
assert_contains "conversation title" "Func Spark" "$(cat /tmp/ft_body)"
assert_contains "conversation confirmation" "resourceUrl" "$(cat /tmp/ft_body)"
assert_contains "conversation webId" "webId" "$(cat /tmp/ft_body)"
CID=$(python3 -c 'import json; print(json.load(open("/tmp/ft_body"))["id"])')
RESURL=$(python3 -c 'import json; print(json.load(open("/tmp/ft_body")).get("resourceUrl",""))')
code=$(curl_code /tmp/ft_body -H "Authorization: Bearer $TOKEN" -H 'Accept: application/ld+json' "$RESURL")
assert_http "conversation jsonld GET" "200" "$code"
assert_contains "jsonld created" "dateCreated" "$(cat /tmp/ft_body)"
body=$(cat /tmp/ft_body)
if [[ "$body" == *2026-09-01T10:15:30Z* || "$body" == *2026-09-01T20:15:30+10:00* ]]; then
  green "PASS  jsonld message time"; PASS=$((PASS+1))
else
  red "FAIL  jsonld message time"; FAIL=$((FAIL+1)); ERRORS+=("jsonld message time")
fi
TTLURL=${RESURL%.json}.ttl
code=$(curl_code /tmp/ft_body -H "Authorization: Bearer $TOKEN" -H 'Accept: text/turtle' "$TTLURL")
assert_http "conversation ttl GET" "200" "$code"
ttlbody=$(cat /tmp/ft_body)
if [[ "$ttlbody" == *dcterms:created* || "$ttlbody" == *purl.org/dc/terms/created* ]]; then
  green "PASS  ttl dcterms created"; PASS=$((PASS+1))
else
  red "FAIL  ttl dcterms created"; FAIL=$((FAIL+1)); ERRORS+=("ttl dcterms created")
fi
code=$(curl_code /tmp/ft_body -X POST "$RESURL" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data '{}')
assert_http "POST to conversation document is 400" "400" "$code"
assert_contains "POST document error" "Can only POST to containers" "$(cat /tmp/ft_body)"
code=$(curl_code /tmp/ft_body "$BASE/conversations" -H "Authorization: Bearer $TOKEN")
assert_http "conversation list" "200" "$code"
assert_contains "conversation list title" "Func Spark" "$(cat /tmp/ft_body)"
code=$(curl_code /tmp/ft_body -X POST "$BASE/conversations/$CID/share" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data '{"public":false}')
assert_http "conversation share" "200" "$code"
SHARE=$(python3 -c 'import json; d=json.load(open("/tmp/ft_body")); print(d["share"]["url"])')
SHAREPATH=${SHARE#*://}
SHAREPATH=/${SHAREPATH#*/}
code=$(curl_code /tmp/ft_body "$SHARE")
assert_http "share url public GET" "200" "$code"
assert_contains "share body" "hello spark" "$(cat /tmp/ft_body)"
code=$(curl_code /tmp/ft_body -X POST "$BASE/conversations/$CID/unshare" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' --data '{}')
assert_http "conversation unshare" "200" "$code"
code=$(curl_code /tmp/ft_body "$SHARE")
if [[ "$code" == "404" || "$code" == "401" ]]; then
  green "PASS  unshare share url $code"; PASS=$((PASS+1))
else
  red "FAIL  unshare share url (expected 404/401 actual=$code)"
  FAIL=$((FAIL+1)); ERRORS+=("unshare share url: $code")
fi

curl -sS -X PUT "$BASE/workspace/n3.ttl" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/turtle' \
  --data '<http://ex/s> <http://ex/p> <http://ex/o1> .' >/dev/null
code=$(curl_code /tmp/ft_body -X PATCH "$BASE/workspace/n3.ttl" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/n3' \
  --data '<http://ex/s> <http://ex/p> <http://ex/o2> .')
assert_http "N3 PATCH" "200" "$code"
assert_contains "N3 patch body" "http://ex/o2" "$(curl -sS "$BASE/workspace/n3.ttl")"

code=$(curl_code /tmp/ft_body -X DELETE "$BASE/workspace/" -H "Authorization: Bearer $TOKEN")
assert_http "delete nonempty container 409" "409" "$code"

code=$(curl_code /tmp/ft_body -X PUT "$BASE/workspace/note.txt" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/plain' \
  -H 'If-None-Match: *' --data 'x')
assert_http "If-None-Match * on existing" "412" "$code"

code=$(curl_code /tmp/ft_body -X PUT "$BASE/workspace/x.txt" \
  -H 'Authorization: Bearer not-a-jwt' -H 'Content-Type: text/plain' --data 'x')
assert_http "invalid bearer 401" "401" "$code"

code=$(curl_code /tmp/ft_body -D /tmp/ft_hdr -X OPTIONS "$BASE/agents" \
  -H 'Origin: http://example.com' -H 'Access-Control-Request-Method: POST')
assert_http "CORS preflight" "204" "$code"
assert_contains "CORS allow origin" "Access-Control-Allow-Origin" "$(cat /tmp/ft_hdr)"

# SSE notification on create
set +e
python3 - <<PY
import threading, urllib.request, time, subprocess, sys
base = "$BASE"
token = "$TOKEN"
got = bytearray()
err = [None]
def reader():
    try:
        with urllib.request.urlopen(urllib.request.Request(base + "/notifications/stream?topic=*"), timeout=5) as r:
            deadline = time.time() + 3
            while time.time() < deadline:
                chunk = r.read(128)
                if chunk:
                    got.extend(chunk)
                    if b"data:" in got:
                        return
                else:
                    time.sleep(0.05)
    except Exception as e:
        err[0] = e
t = threading.Thread(target=reader, daemon=True)
t.start()
time.sleep(0.4)
subprocess.call([
    "curl", "-s", "-X", "PUT", base + "/workspace/sse.txt",
    "-H", "Authorization: Bearer " + token,
    "-H", "Content-Type: text/plain", "--data", "sse-ping",
], stdout=subprocess.DEVNULL)
t.join(4)
ok = b"data:" in got
print("SSE_OK" if ok else f"SSE_FAIL err={err[0]} bytes={len(got)}")
sys.exit(0 if ok else 1)
PY
sse_rc=$?
set -e
if [[ $sse_rc -eq 0 ]]; then
  green "PASS  SSE notification"; PASS=$((PASS+1))
else
  red "FAIL  SSE notification"; FAIL=$((FAIL+1)); ERRORS+=("SSE notification")
fi

echo
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ $FAIL -gt 0 ]]; then
  echo "Failures:"
  for e in "${ERRORS[@]}"; do echo "  - $e"; done
  exit 1
fi
exit 0
