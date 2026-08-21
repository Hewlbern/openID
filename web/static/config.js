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
})(window);
