(function (g) {
  if (typeof g.OPENID_API !== "string") {
    g.OPENID_API = "";
  }
  g.openidURL = function (path) {
    if (!path) return String(g.OPENID_API || "").replace(/\/$/, "");
    if (/^https?:\/\//i.test(path)) return path;
    var base = String(g.OPENID_API || "").replace(/\/$/, "");
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
