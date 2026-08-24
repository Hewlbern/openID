#!/usr/bin/env bash
# Start the local OpenID pod grokbot uses (dashboard + MCP on :4000).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
mkdir -p "$ROOT/data" "$ROOT/bin"
go build -o "$ROOT/bin/openid" ./cmd/server
go build -o "$ROOT/bin/openid-mcp" ./cmd/mcp
export SOLID_TOKEN_SECRET="${SOLID_TOKEN_SECRET:-openid-local-pod}"
exec "$ROOT/bin/openid" -port 4000 -storage "$ROOT/data" -base-url http://localhost:4000
