import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createContext, runInContext } from "node:vm";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");

function sparkTokenFetchSource() {
  const app = readFileSync(join(root, "web/static/app.js"), "utf8");
  const match = app.match(/async function sparkTokenFetch\([\s\S]*?\n\}/);
  assert.ok(match, "sparkTokenFetch must exist in web/static/app.js");
  return match[0];
}

function loadOpenidFetch() {
  const src = readFileSync(join(root, "web/static/config.js"), "utf8");
  const captured = [];
  const store = { "openid.token": "session-jwt" };
  const sandbox = {
    window: {},
    localStorage: {
      getItem: (k) => store[k] || "",
    },
    fetch: (url, opts) => {
      captured.push({ url, opts });
      return Promise.resolve({ ok: true, status: 200 });
    },
  };
  sandbox.window = sandbox;
  runInContext(src, createContext(sandbox));
  return { sandbox, captured };
}

test("sparkTokenFetch uses openidFetch so Authorization is not overwritten", () => {
  const src = sparkTokenFetchSource();
  assert.match(src, /openidFetch\(/);
  assert.doesNotMatch(
    src,
    /Object\.assign\(\s*\{\s*headers:\s*openidHeaders/,
    "must not Object.assign({ headers: openidHeaders(...) }, opts) — opts.headers wins and drops Authorization"
  );
});

test("openidFetch keeps session Bearer when mint POST passes Content-Type", async () => {
  const { sandbox, captured } = loadOpenidFetch();
  const opts = { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" };
  await sandbox.openidFetch("/api/spark-token", opts);
  assert.equal(captured.length, 1);
  assert.equal(captured[0].opts.headers.Authorization, "Bearer session-jwt");
  assert.equal(captured[0].opts.headers["Content-Type"], "application/json");
  assert.equal(captured[0].opts.method, "POST");
});

test("buggy Object.assign overwrite drops Authorization (documents the regression)", () => {
  const session = { Accept: "application/json", Authorization: "Bearer session-jwt" };
  const opts = { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" };
  const broken = Object.assign({
    headers: session,
    credentials: "include",
  }, opts);
  assert.equal(broken.headers.Authorization, undefined);
  assert.equal(broken.headers["Content-Type"], "application/json");

  const fixed = Object.assign({}, opts, {
    headers: Object.assign({ Accept: "application/json" }, opts.headers, { Authorization: "Bearer session-jwt" }),
    credentials: "include",
  });
  assert.equal(fixed.headers.Authorization, "Bearer session-jwt");
  assert.equal(fixed.headers["Content-Type"], "application/json");
});
