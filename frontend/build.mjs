import { cpSync, mkdirSync, writeFileSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const src = join(root, "..", "web", "static");
const dist = join(root, "dist");
const pod = (process.env.OPENID_POD || process.env.OPENID_API || process.env.NEXT_PUBLIC_SOLID_URL || "https://pod-production-ebe1.up.railway.app").replace(/\/$/, "");
// Browser talks to this Vercel origin; API routes are rewritten to the pod.
// Leave OPENID_API empty so register/login/session stay same-origin.
const browserAPI = process.env.OPENID_BROWSER_API === "1" ? pod : "";

mkdirSync(join(dist, "static"), { recursive: true });
cpSync(src, join(dist, "static"), { recursive: true });

for (const name of readdirSync(src)) {
  if (name.endsWith(".html")) {
    cpSync(join(src, name), join(dist, name));
  }
}

const config = `(function (g) {
  g.OPENID_API = ${JSON.stringify(browserAPI)};
  g.openidURL = function (path) {
    if (!path) return String(g.OPENID_API || "").replace(/\\/$/, "");
    if (/^https?:\\/\\//i.test(path)) return path;
    var base = String(g.OPENID_API || "").replace(/\\/$/, "");
    return base + (path.charAt(0) === "/" ? path : "/" + path);
  };
  g.openidHeaders = function (extra) {
    var h = Object.assign({ Accept: "application/json" }, extra || {});
    var tok = "";
    try { tok = localStorage.getItem("openid.token") || ""; } catch (e) {}
    if (tok && !h.Authorization) h.Authorization = "Bearer " + tok;
    return h;
  };
  g.openidFetch = function (path, opts) {
    opts = opts || {};
    opts.credentials = opts.credentials || "include";
    opts.headers = g.openidHeaders(opts.headers);
    return fetch(g.openidURL(path), opts);
  };
})(window);
`;
writeFileSync(join(dist, "static", "config.js"), config);
writeFileSync(join(dist, "config.js"), config);

const proxy = (source) => ({ source, destination: pod + source.replace(/:path\*/g, ":path*") });
const vercel = {
  rewrites: [
    { source: "/dashboard", destination: "/dash.html" },
    { source: "/dashboard/", destination: "/dash.html" },
    { source: "/welcome", destination: "/index.html" },
    { source: "/welcome/", destination: "/index.html" },
    { source: "/app", destination: "/app.html" },
    { source: "/app/", destination: "/app.html" },
    { source: "/app/:path*", destination: "/app.html" },
    { source: "/records", destination: "/records.html" },
    { source: "/records/", destination: "/records.html" },
    { source: "/login", destination: "/dash.html" },
    { source: "/idp/:path*", destination: `${pod}/idp/:path*` },
    { source: "/oauth/:path*", destination: `${pod}/oauth/:path*` },
    { source: "/conversations", destination: `${pod}/conversations` },
    { source: "/conversations/:path*", destination: `${pod}/conversations/:path*` },
    { source: "/share/c/:path*", destination: `${pod}/share/c/:path*` },
    { source: "/mcp", destination: `${pod}/mcp` },
    { source: "/mcp/:path*", destination: `${pod}/mcp/:path*` },
    { source: "/agents", destination: `${pod}/agents` },
    { source: "/agents/:path*", destination: `${pod}/agents/:path*` },
    { source: "/audit/:path*", destination: `${pod}/audit/:path*` },
    { source: "/notifications/:path*", destination: `${pod}/notifications/:path*` },
    { source: "/health", destination: `${pod}/health` },
    { source: "/api/:path*", destination: `${pod}/api/:path*` },
    { source: "/.well-known/:path*", destination: `${pod}/.well-known/:path*` },
    { source: "/i/:path*", destination: `${pod}/i/:path*` },
    { source: "/:path*", destination: `${pod}/:path*` },
  ],
};
void proxy;
writeFileSync(join(dist, "vercel.json"), JSON.stringify(vercel, null, 2));

const index = readFileSync(join(dist, "index.html"), "utf8");
if (!index.includes("config.js")) {
  writeFileSync(join(dist, "index.html"), index.replace("</head>", '  <script src="/static/config.js"></script>\n</head>'));
}

console.log("frontend build →", dist, "pod=", pod, "browser OPENID_API=", browserAPI || "(same origin, proxied)");
