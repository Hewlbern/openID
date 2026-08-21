<p align="center">
  <img src="assets/openid-icon.png" alt="OpenID icon" width="120" height="120" />
</p>

<h1 align="center">OpenID</h1>

<p align="center">
  <strong>Solid Protocol identity for AI agents</strong><br/>
  WebID pods · WAC access control · IPFS content addressing · Bitcoin-anchored audit via OpenTimestamps
</p>

<p align="center">
  <a href="https://github.com/Hewlbern/openID"><img alt="GitHub" src="https://img.shields.io/badge/github-Hewlbern%2FopenID-0B1F33?style=flat-square" /></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache%202.0-1FA7A0?style=flat-square" /></a>
  <a href="#quick-start"><img alt="Go" src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" /></a>
</p>

---

OpenID is a [Solid Protocol](https://solidproject.org/TR/protocol) server written in Go. It gives each AI agent a **WebID**, a **pod**, and a **tamper-evident audit trail**: every mutating action is hashed, pinned to IPFS, Merkle-batched, and stamped with [OpenTimestamps](https://opentimestamps.org/) (Bitcoin calendar attestation).

> Not the same as [OpenID Connect](https://openid.net/connect/) the SSO standard — though this server also exposes OIDC discovery and client-credentials for machine agents. Here **OpenID** means *open identity* for agents on Solid.

## Why

| Need | What OpenID provides |
|------|----------------------|
| Agent identity | Solid WebID + Ed25519 key + optional `did:key` |
| Private data store | LDP pod with WAC ACLs |
| Interop | HTTP Linked Data (Turtle, SPARQL UPDATE, N3-Patch) |
| Proof of action | IPFS CID + Merkle path + OTS Bitcoin calendar proof |
| Machine auth | Bearer JWT, DPoP, client-credentials, agent signatures |

## Quick start

```bash
git clone https://github.com/Hewlbern/openID.git
cd openID
go run ./cmd/server -port 3000 -storage ./data
```

Open the Solid server dashboard (this is the server console, not the marketing landing):

```
http://localhost:3000/
```

Handle-claim landing (separate product page):

```
http://localhost:3000/welcome
```

Health check:

```bash
curl -s http://localhost:3000/health
# {"status":"ok"}
```

MCP (for Grok Bot / Cursor): `http://localhost:3000/mcp` — also stdio via `go run ./cmd/mcp` with `OPENID_BASE_URL`.

```bash
curl -s http://localhost:3000/mcp
# {"open":true,"dashboard":"...","mcp":".../mcp", ...}
```

Claim a handle:

```bash
curl -s http://localhost:3000/idp/handles/ada
curl -s -X POST http://localhost:3000/idp/register \
  -H 'Content-Type: application/json' \
  -d '{"handle":"ada","password":"testpass123","name":"Ada"}'
# Public page: http://localhost:3000/i/ada
# Passport UI: http://localhost:3000/app
```

## Hosted deploy

The Solid server is a long-lived Go process with a volume. The marketing UI can sit on Vercel and call that origin.

**Railway (pod origin)**

1. Create a new Railway project from this repo (`Dockerfile` + `railway.toml`).
2. Attach a volume at `/data`.
3. Set `SOLID_STORAGE_PATH=/data`, a real `SOLID_TOKEN_SECRET`, and `SOLID_BASE_URL=https://<your-railway-domain>` after the domain exists.
4. Railway injects `PORT`; the binary already reads it.

**Vercel (frontend)**

```bash
cd frontend
OPENID_API=https://<your-railway-domain> vercel --prod
```

Other operators can run the same split: one SolidGo instance (Railway/Docker) plus a Vercel project whose `OPENID_API` points at their server.

With Docker (server + Kubo IPFS):

```bash
docker compose up -d --build
```

## 60-second agent demo

```bash
# 1. Register an agent → WebID, keypair, Bearer token
curl -s -X POST http://localhost:3000/agents \
  -H 'Content-Type: application/json' \
  -d '{"name":"Atlas"}' | jq

# 2. Write to the agent pod (audited)
TOKEN=<token from step 1>
POD=agent-<id>/
curl -s -X PUT "http://localhost:3000/${POD}inbox/task.json" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"task":"summarize"}'

# 3. Anchor audit batch (Merkle root → OpenTimestamps)
curl -s -X POST http://localhost:3000/audit/flush | jq

# 4. Verify an event
curl -s "http://localhost:3000/audit/events/<event-id>/verify" | jq
```

### Agent request signing

Instead of a Bearer token, agents can sign each request:

| Header | Value |
|--------|--------|
| `X-Agent-WebID` | Agent WebID |
| `X-Agent-Public-Key` | base64url Ed25519 public key |
| `X-Agent-Timestamp` | Unix seconds |
| `X-Agent-Signature` | Ed25519 over `METHOD\|PATH\|TIMESTAMP\|WEBID` |

## Architecture

```mermaid
sequenceDiagram
    participant Agent as AI_Agent
    participant OpenID as OpenID_Server
    participant IPFS as IPFS_Kubo
    participant OTS as OpenTimestamps
    participant BTC as Bitcoin

    Agent->>OpenID: Register agent pod plus Ed25519 pubkey
    OpenID->>OpenID: Mint WebID profile and ACL
    Agent->>OpenID: Signed action on resource
    OpenID->>OpenID: Verify sig against WebID key
    OpenID->>OpenID: Append audit event hash
    OpenID->>IPFS: Pin event payload CID
    OpenID->>OpenID: Batch event hashes into Merkle root
    OpenID->>OTS: Stamp Merkle root
    OTS->>BTC: Calendar anchors to Bitcoin
    Agent->>OpenID: GET audit proof for action
    OpenID->>Agent: CID plus OTS proof plus Merkle path
```

When OTS calendars are unreachable, a **pending local proof** is stored and can be upgraded later via verify.

## API surface

### Identity

| Endpoint | Purpose |
|----------|---------|
| `GET/POST /mcp` | MCP for Grok Bot (open, tools/list, tools/call) |
| `POST /agents` | Register AI agent (pod + WebID + keys) |
| `GET /agents` | List agents |
| `POST /idp/register` | Human account + pod |
| `POST /idp/login` | Password login |
| `POST /idp/client-credentials` | Machine client id/secret |
| `POST /oauth/token` | Token endpoint |
| `GET /.well-known/openid-configuration` | OIDC discovery |
| `GET /.well-known/solid` | Solid storage description |

### Solid LDP

`GET` `HEAD` `PUT` `POST` `PATCH` `DELETE` `OPTIONS` on resource paths.

- Containers end with `/` and return Turtle `ldp:contains`
- Create containers with `Link: <http://www.w3.org/ns/ldp#BasicContainer>; rel="type"`
- Patch with `Content-Type: application/sparql-update` or `text/n3`
- ACLs at `{resource}.acl` or `{container}/.acl`

### Audit

| Endpoint | Purpose |
|----------|---------|
| `GET /audit/events/` | List audit events |
| `GET /audit/events/{id}` | Event detail (includes IPFS CID) |
| `GET /audit/events/{id}/verify` | Hash + Merkle + OTS check |
| `POST /audit/flush` | Force Merkle batch + OTS stamp |
| `GET /audit/batches/{id}` | Batch + Merkle paths + OTS proof |

### Notifications

| Endpoint | Purpose |
|----------|---------|
| `GET /notifications/websocket?topic=` | WebSocket activity stream |
| `GET /notifications/stream?topic=` | SSE activity stream |

## Configuration

| Flag / env | Default | Meaning |
|------------|---------|---------|
| `-port` / `SOLID_PORT` | `3000` | Listen port |
| `-storage` / `SOLID_STORAGE_PATH` | `./data` | File storage root |
| `-base-url` / `SOLID_BASE_URL` | `http://localhost:{port}` | Public base URL for WebIDs |
| `-ipfs-api` / `IPFS_API` | _(empty = offline CIDs)_ | Kubo HTTP API |
| `SOLID_TOKEN_SECRET` | dev default | JWT HMAC secret |
| `AUDIT_BATCH_INTERVAL` | `30s` | Merkle/OTS batch period |
| `OTS_CALENDAR` | public OTS calendars | Comma-separated calendar URLs |

## Develop

```bash
go test ./internal/... ./test/...
go build -o openid ./cmd/server
docker compose --profile test run --rm solid-tests
```

### Functional tests (live HTTP)

```bash
go build -o /tmp/openid ./cmd/server
/tmp/openid -port 3460 -storage /tmp/openid-data -base-url http://localhost:3460 &
BASE_URL=http://localhost:3460 ./test/functional/run.sh
```

Covers health, OIDC discovery, register/login, LDP CRUD, ETags/conditions, SPARQL + N3 patch, WAC, client-credentials, AI agent + Ed25519 signatures, audit Merkle/OTS verify, SSE notifications, CORS, and the grokbot MCP (HTTP + stdio).

```bash
BASE_URL=http://localhost:3460 ./test/functional/mcp.sh
```

Grok Bot / Cursor load `.cursor/mcp.json` and `.mcp.json` in this repo (`url: http://127.0.0.1:4000/mcp` when the local pod is on port 4000).

## Layout

```
assets/openid-icon.png    brand mark
cmd/server/               entrypoint
internal/solid/           LDP HTTP handler
internal/resourcestore/   containers, ETags, metadata
internal/wac/             Web Access Control
internal/authn/           Bearer / DPoP / agent signatures
internal/identityapi/     accounts, OIDC, client credentials
internal/agent/           AI agent registry
internal/audit/           hash chain + Merkle batches
internal/ipfs/            Kubo client (+ offline fallback)
internal/ots/             OpenTimestamps client
internal/notify/          WebSocket + SSE
internal/openidmcp/       MCP stdio + HTTP for Grok Bot
internal/rdf/             Turtle / SPARQL UPDATE / N3-Patch
cmd/mcp/                  stdio MCP entrypoint
```

## License

Apache-2.0 — see [LICENSE](LICENSE).

Author: [Michael Holborn](https://www.linkedin.com/in/michaelholborn/)
