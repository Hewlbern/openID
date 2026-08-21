import { cpSync, mkdirSync, writeFileSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const src = join(root, "..", "web", "static");
const dist = join(root, "dist");
const api = (process.env.OPENID_API || process.env.NEXT_PUBLIC_SOLID_URL || "").replace(/\/$/, "");

mkdirSync(join(dist, "static"), { recursive: true });
cpSync(src, join(dist, "static"), { recursive: true });

for (const name of readdirSync(src)) {
  if (name.endsWith(".html")) {
    cpSync(join(src, name), join(dist, name));
  }
}

const config = `(function (g) {
  g.OPENID_API = ${JSON.stringify(api)};
  g.openidURL = function (path) {
    if (!path) return String(g.OPENID_API || "").replace(/\\/$/, "");
    if (/^https?:\\/\\//i.test(path)) return path;
    var base = String(g.OPENID_API || "").replace(/\\/$/, "");
    return base + (path.charAt(0) === "/" ? path : "/" + path);
  };
})(window);
`;
writeFileSync(join(dist, "static", "config.js"), config);
writeFileSync(join(dist, "config.js"), config);

const index = readFileSync(join(dist, "index.html"), "utf8");
if (!index.includes("config.js")) {
  writeFileSync(join(dist, "index.html"), index.replace("</head>", '  <script src="/static/config.js"></script>\n</head>'));
}

console.log("frontend build →", dist, "OPENID_API=", api || "(same origin)");
