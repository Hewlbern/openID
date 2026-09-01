import assert from "node:assert/strict";
import { createServer } from "node:http";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const require = createRequire(import.meta.url);

function loadMcp() {
  for (const rel of [
    "api/mcp.js",
    "api/_lib/pod.js",
    "api/_lib/jwt.js",
    "api/_lib/share.js",
    "api/_lib/spark-mint.js",
  ]) {
    const abs = require.resolve(join(root, rel));
    delete require.cache[abs];
  }
  return require(join(root, "api/mcp.js"));
}

function invoke(handler, { method = "POST", headers = {}, body = {}, query = {} } = {}) {
  return new Promise((resolve) => {
    const req = { method, headers, body, query, url: "/api/mcp" };
    const res = {
      statusCode: 200,
      headers: {},
      setHeader(k, v) { this.headers[k] = v; },
      status(code) { this.statusCode = code; return this; },
      json(obj) { resolve({ status: this.statusCode, headers: this.headers, body: obj }); },
      end(text) { resolve({ status: this.statusCode, headers: this.headers, body: text ?? "" }); },
    };
    handler(req, res);
  });
}

function rpc(method, params, id = 1) {
  return { jsonrpc: "2.0", id, method, params };
}

test("tools/list includes spark_login and spark_register", async () => {
  const handler = loadMcp();
  const tools = handler.sparkTools().map((t) => t.name);
  assert.ok(tools.includes("spark_login"));
  assert.ok(tools.includes("spark_register"));
  assert.ok(tools.includes("spark_save_conversation"));
  assert.ok(tools.includes("spark_share_conversation"));
  const login = handler.sparkTools().find((t) => t.name === "spark_login");
  assert.match(login.description, /handle/i);
  assert.match(login.description, /password/i);
  assert.ok(login.inputSchema.properties.password);
  assert.match(login.description, /Never echo the password/);
});

test("initialize instructions tell the model to spark_login then save then share", async () => {
  const handler = loadMcp();
  assert.match(handler.SPARK_INSTRUCTIONS, /spark_login/);
  assert.match(handler.SPARK_INSTRUCTIONS, /spark_save_conversation/);
  assert.match(handler.SPARK_INSTRUCTIONS, /spark_share_conversation/);
  const out = await invoke(handler, { body: rpc("initialize", { protocolVersion: "2025-03-26" }) });
  assert.equal(out.status, 200);
  assert.match(out.body.result.instructions, /spark_login/);
  assert.match(out.body.result.instructions, /Authorization: Bearer/);
});

test("spark_login mints a connect token and never echoes the password", async () => {
  const prev = process.env.OPENID_POD;
  const calls = [];
  const server = createServer((req, res) => {
    const url = new URL(req.url, "http://pod.test");
    let raw = "";
    req.on("data", (c) => { raw += c; });
    req.on("end", () => {
      calls.push({ method: req.method, path: url.pathname, auth: req.headers.authorization || "", body: raw });
      res.setHeader("Content-Type", "application/json");
      if (req.method === "POST" && url.pathname === "/idp/login") {
        const body = JSON.parse(raw || "{}");
        if (body.password !== "s3cret-pass" || body.handle !== "ada") {
          res.statusCode = 401;
          res.end("invalid credentials");
          return;
        }
        res.end(JSON.stringify({
          token: "session-jwt-ada",
          webId: "https://pod.test/ada/profile/card#me",
          account: { handle: "ada", webId: "https://pod.test/ada/profile/card#me" },
        }));
        return;
      }
      if (url.pathname === "/idp/accounts/me") {
        res.end(JSON.stringify({ handle: "ada", webId: "https://pod.test/ada/profile/card#me" }));
        return;
      }
      if (url.pathname.endsWith("/") || url.pathname.includes("/.openid/")) {
        res.statusCode = req.method === "GET" ? 404 : 201;
        res.end(req.method === "GET" ? "" : "{}");
        return;
      }
      res.statusCode = 404;
      res.end("{}");
    });
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const { port } = server.address();
  process.env.OPENID_POD = "http://127.0.0.1:" + port;
  try {
    const handler = loadMcp();
    const out = await invoke(handler, {
      headers: { host: "preview.example", "x-forwarded-proto": "https", "x-forwarded-host": "preview.example" },
      body: rpc("tools/call", {
        name: "spark_login",
        arguments: { handle: "ada", password: "s3cret-pass" },
      }),
    });
    assert.equal(out.status, 200);
    assert.equal(out.body.result.isError, undefined);
    const parsed = JSON.parse(out.body.result.content[0].text);
    assert.equal(parsed.ok, true);
    assert.equal(parsed.handle, "ada");
    assert.equal(parsed.webId, "https://pod.test/ada/profile/card#me");
    assert.equal(parsed.tokenType, "Bearer");
    assert.equal(parsed.mcpUrl, "https://preview.example/mcp");
    assert.ok(parsed.token && parsed.token.split(".").length === 3);
    assert.match(parsed.hint, /Bearer/);
    assert.doesNotMatch(JSON.stringify(parsed), /s3cret-pass/);
    assert.doesNotMatch(JSON.stringify(out.body), /s3cret-pass/);
    const loginCall = calls.find((c) => c.path === "/idp/login");
    assert.ok(loginCall, "must POST /idp/login");
    assert.match(loginCall.body, /s3cret-pass/);
  } finally {
    if (prev === undefined) delete process.env.OPENID_POD;
    else process.env.OPENID_POD = prev;
    await new Promise((r) => server.close(r));
  }
});

test("spark_login without handle or password is an error", async () => {
  const handler = loadMcp();
  const out = await invoke(handler, {
    body: rpc("tools/call", { name: "spark_login", arguments: { password: "x" } }),
  });
  assert.equal(out.status, 200);
  assert.equal(out.body.result.isError, true);
  assert.match(out.body.result.content[0].text, /handle/);
});

test("Bearer Authorization header path remains documented on spark_* tools", async () => {
  const { issueSparkToken, isSparkTokenShape } = require(join(root, "api/_lib/jwt.js"));
  const minted = issueSparkToken({
    webId: "https://pod.test/ada/profile/card#me",
    handle: "ada",
    sessionToken: "session-jwt-ada",
  });
  assert.ok(isSparkTokenShape(minted.token));
  const handler = loadMcp();
  const listed = await invoke(handler, { body: rpc("tools/list") });
  const save = listed.body.result.tools.find((t) => t.name === "spark_save_conversation");
  assert.ok(save.inputSchema.properties.token);
  assert.match(save.description, /spark_login/);
  assert.match(save.inputSchema.properties.token.description, /Authorization: Bearer/);
});
