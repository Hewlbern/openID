#!/usr/bin/env python3
"""Save Grok Bot + Cursor records as JSON-LD / RDF on the OpenID pod."""
from __future__ import annotations

import json
import os
import re
import time
import urllib.error
import urllib.request
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path

BASE = os.environ.get("BASE_URL", "http://localhost:4000").rstrip("/")
HANDLE = os.environ.get("OPENID_HANDLE", "mike")
PASSWORD = os.environ.get("OPENID_PASSWORD", "")
ROOT = f"{HANDLE}/records"
NS = f"{BASE}/ns/records#"
WEBID = f"{BASE}/{HANDLE}/profile/card#me"

SECRET_RE = re.compile(
    r"(eyJ[A-Za-z0-9_\-]{20,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,})"
    r"|(sk-[A-Za-z0-9]{20,})|(xai-[A-Za-z0-9]{20,})|(SG\.[A-Za-z0-9_\-\.]{20,})"
    r"|((?:Bearer|token|secret|password|api[_-]?key)\s*[:=]\s*\S+)",
    re.I,
)
SECRET_KEYS = {
    "token", "access_token", "secret", "client_secret", "password", "api_key",
    "apikey", "authorization", "privatekey", "private_key", "credential",
    "headers", "encrypted", "authinfo", "x-anyrun-network-token",
}

CONTEXT = {
    "@vocab": "https://schema.org/",
    "oid": NS,
    "dcat": "http://www.w3.org/ns/dcat#",
    "dct": "http://purl.org/dc/terms/",
    "foaf": "http://xmlns.com/foaf/0.1/",
    "solid": "http://www.w3.org/ns/solid/terms#",
    "xsd": "http://www.w3.org/2001/XMLSchema#",
    "workType": "oid:workType",
    "domain": "oid:domain",
    "package": "oid:package",
    "artifact": "oid:artifact",
    "project": "oid:project",
    "userTurns": {"@id": "oid:userTurns", "@type": "xsd:integer"},
    "assistantTurns": {"@id": "oid:assistantTurns", "@type": "xsd:integer"},
    "sourcePath": "oid:sourcePath",
    "parentTrace": {"@id": "oid:parentTrace", "@type": "@id"},
}

DOMAIN_RULES = [
    ("agent-identity", ("identity", "openid", "solid", "webid", "oidc", "grokbot")),
    ("field-operations", ("frontline", "front-line", "frontlinenative")),
    ("creator-media", ("dreammachine", "dream-machina", "nori", "storyframe", "story-story")),
    ("consumer-hardware", ("swellbot", "swell.melbourne")),
    ("affiliate-growth", ("affiliate", "ourasociety")),
    ("blockchain", ("blockchain", "optimism", "wallet-track")),
    ("knowledge-graph", ("knowledge-graph", "llm-graph", "graph-builder", "/kg")),
    ("animation-tools", ("aidol", "vaidol", "anime", "glyphic")),
    ("personal-site", ("portfolio", "resume", "buzz")),
    ("software-platform", ("aro", "tomoro", "expo", "tattd", "moltbot", "wiseclaw", "processors")),
]

WORK_RULES = [
    ("identity-protocol", ("openid", "solid", "webid", "oidc", "mcp", "pod", "json-ld", "rdf", "jsonld")),
    ("debugging", ("fix", "bug", "error", "fail", "broken", "why does", "not work", "401", "403")),
    ("deployment", ("deploy", "railway", "vercel", "domain", "dns", "docker")),
    ("marketing", ("landing", "copy", "seo", "brand", "preorder", "checkout", "stripe", "video")),
    ("architecture", ("architect", "design", "plan", "how should", "trade-off")),
    ("creative", ("generate", "image", "animate", "nori", "video", "mockup")),
    ("research", ("investigate", "compare", "what is", "audit", "review")),
    ("implementation", ("implement", "build", "add", "make sure", "wire", "create", "test")),
]

PACKAGE_FOR_DOMAIN = {
    "agent-identity": "openid-agent-identity",
    "field-operations": "frontline-field-product",
    "creator-media": "dreammachine-studio",
    "consumer-hardware": "swellbot-hardware",
    "affiliate-growth": "affiliate-growth",
    "blockchain": "onchain-research",
    "knowledge-graph": "knowledge-graph-work",
    "animation-tools": "animation-tools",
    "personal-site": "personal-web",
    "software-platform": "software-platform",
    "general": "uncategorized-agent-work",
}

PACKAGE_META = {
    "openid-agent-identity": {
        "name": "OpenID agent identity corpus",
        "description": "Solid/WebID, MCP, Grok Bot, and agent-pod implementation traces.",
        "audience": "AI infrastructure buyers",
    },
    "frontline-field-product": {
        "name": "Frontline field-product corpus",
        "description": "Product, mobile, and operations traces for a field-service company.",
        "audience": "Ops and product studios",
    },
    "dreammachine-studio": {
        "name": "Dream Machine studio corpus",
        "description": "Brand, video, and creator-tool traces.",
        "audience": "Creative studios",
    },
    "swellbot-hardware": {
        "name": "swellBot hardware-site corpus",
        "description": "Consumer hardware site, checkout, and launch traces.",
        "audience": "Hardware marketers",
    },
    "affiliate-growth": {
        "name": "Affiliate growth corpus",
        "description": "SEO and affiliate-site traces.",
        "audience": "Growth teams",
    },
    "onchain-research": {
        "name": "Onchain research corpus",
        "description": "Blockchain and wallet-tracking traces.",
        "audience": "Crypto researchers",
    },
    "knowledge-graph-work": {
        "name": "Knowledge-graph corpus",
        "description": "Graph builder and RDF resume traces.",
        "audience": "Data platform teams",
    },
    "animation-tools": {
        "name": "Animation tools corpus",
        "description": "Character and animation product traces.",
        "audience": "Creative tool companies",
    },
    "personal-web": {
        "name": "Personal web corpus",
        "description": "Portfolio, resume, and small-site traces.",
        "audience": "Studios building personal brands",
    },
    "software-platform": {
        "name": "Software platform corpus",
        "description": "Aro, Expo, Tomoro, and related product traces.",
        "audience": "Product engineering teams",
    },
    "uncategorized-agent-work": {
        "name": "Uncategorized agent work",
        "description": "Traces that did not match a product domain.",
        "audience": "General agent-workflow researchers",
    },
}


def redact_text(s: str, limit: int | None = None) -> str:
    if not s:
        return ""
    s = SECRET_RE.sub("[REDACTED]", s)
    return s[:limit] + "…" if limit and len(s) > limit else s


def redact_obj(obj):
    if isinstance(obj, dict):
        return {
            k: "[REDACTED]" if k.lower() in SECRET_KEYS or "secret" in k.lower() or "token" in k.lower()
            else redact_obj(v)
            for k, v in obj.items()
        }
    if isinstance(obj, list):
        return [redact_obj(x) for x in obj]
    if isinstance(obj, str):
        return redact_text(obj, 4000)
    return obj


def iso(ts: float) -> str:
    return datetime.fromtimestamp(ts, tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def match_rule(blob: str, rules, default: str) -> str:
    for name, keys in rules:
        if any(k in blob for k in keys):
            return name
    return default


def keywords(query: str, domain: str, work: str) -> list[str]:
    words = re.findall(r"[a-z0-9][a-z0-9\-]{3,}", (query or "").lower())
    stop = {"this", "that", "with", "from", "have", "make", "sure", "want", "just", "then", "they", "your", "about"}
    out = [domain, work]
    for w in words:
        if w not in stop and w not in out:
            out.append(w)
        if len(out) >= 10:
            break
    return out


def classify(project: str, query: str) -> dict:
    blob = f"{project} {query}".lower()
    domain = match_rule(blob, DOMAIN_RULES, "general")
    work = match_rule(blob, WORK_RULES, "implementation" if query else "operations")
    package = PACKAGE_FOR_DOMAIN.get(domain, "uncategorized-agent-work")
    return {
        "domain": domain,
        "workType": work,
        "package": package,
        "keywords": keywords(query, domain, work),
    }


def offer_for(item: dict) -> dict:
    turns = int(item.get("userTurns") or 0)
    price = 9 if item.get("artifact") == "Conversation" else 3
    price += min(20, turns // 10)
    if item.get("workType") in ("identity-protocol", "architecture"):
        price += 8
    if item.get("domain") == "agent-identity":
        price += 5
    return {
        "@type": "Offer",
        "name": "Paid download (not listed yet)",
        "availability": "https://schema.org/InStoreOnly",
        "businessFunction": "https://schema.org/Sell",
        "priceCurrency": "USD",
        "price": price,
        "seller": {"@id": WEBID},
        "itemOffered": {"@id": item["@id"]},
    }


def login() -> str:
    req = urllib.request.Request(
        BASE + "/idp/login",
        data=json.dumps({"handle": HANDLE, "password": PASSWORD}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req) as r:
        return json.load(r)["token"]


def put(token: str, path: str, body: bytes, ctype: str) -> int:
    req = urllib.request.Request(
        f"{BASE}/{path.lstrip('/')}",
        data=body,
        method="PUT",
        headers={"Authorization": "Bearer " + token, "Content-Type": ctype},
    )
    try:
        with urllib.request.urlopen(req) as r:
            return r.status
    except urllib.error.HTTPError as e:
        if e.code in (200, 201, 204):
            return e.code
        raise SystemExit(f"PUT {path} -> {e.code} {e.read()[:240]}")


def put_jsonld(token: str, path: str, obj) -> int:
    return put(token, path, json.dumps(obj, indent=2, default=str).encode(), "application/ld+json")


def put_container(token: str, path: str) -> None:
    if not path.endswith("/"):
        path += "/"
    req = urllib.request.Request(
        f"{BASE}/{path}",
        method="PUT",
        headers={
            "Authorization": "Bearer " + token,
            "Link": '<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"',
        },
    )
    try:
        urllib.request.urlopen(req).read()
    except urllib.error.HTTPError as e:
        if e.code not in (200, 201, 204, 409, 412):
            raise SystemExit(f"container {path} -> {e.code}")


def first_user_query(path: Path) -> str:
    try:
        with path.open(encoding="utf-8", errors="replace") as f:
            for line in f:
                if '"role":"user"' not in line and '"role": "user"' not in line:
                    continue
                try:
                    rec = json.loads(line)
                except Exception:
                    continue
                msg = rec.get("message") or rec.get("content") or {}
                chunks = []
                if isinstance(msg, dict):
                    content = msg.get("content") or []
                    if isinstance(content, str):
                        chunks.append(content)
                    elif isinstance(content, list):
                        for c in content:
                            if isinstance(c, dict) and c.get("type") in (None, "text"):
                                chunks.append(c.get("text") or "")
                text = "\n".join(chunks)
                m = re.search(r"<user_query>\s*(.*?)\s*</user_query>", text, re.S)
                if m:
                    return redact_text(m.group(1).strip(), 400)
                text = re.sub(r"<external_links>.*?</external_links>", "", text, flags=re.S)
                text = re.sub(r"<[^>]+>", " ", text)
                text = re.sub(r"\s+", " ", text).strip()
                if text:
                    return redact_text(text, 400)
    except Exception:
        return ""
    return ""


def count_turns(path: Path) -> dict:
    users = assistants = 0
    with path.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            if '"role":"user"' in line or '"role": "user"' in line:
                users += 1
            elif '"role":"assistant"' in line or '"role": "assistant"' in line:
                assistants += 1
    return {"userTurns": users, "assistantTurns": assistants}


def collect_traces() -> list[dict]:
    root = Path("/Users/mikeholborn/.cursor/projects")
    items = []
    for p in root.rglob("agent-transcripts/**/*.jsonl"):
        st = p.stat()
        project = p.parts[p.parts.index("projects") + 1]
        query = first_user_query(p)
        kind = "subagent" if "/subagents/" in str(p) else "conversation"
        parent = p.parent.parent.name if kind == "subagent" else None
        facets = classify(project, query)
        turns = count_turns(p)
        iri = f"{BASE}/{ROOT}/traces/{facets['package']}/{p.stem}.jsonld"
        item = {
            "@context": CONTEXT,
            "@id": iri,
            "@type": ["CreativeWork", "Dataset", "oid:AgentTrace"],
            "identifier": p.stem,
            "name": (query[:80] or p.stem),
            "description": query,
            "creator": {"@id": WEBID, "@type": "Person", "name": "Mike Holborn"},
            "dateCreated": iso(st.st_ctime),
            "dateModified": iso(st.st_mtime),
            "encodingFormat": "application/x-ndjson",
            "contentSize": str(st.st_size),
            "inLanguage": "en",
            "genre": facets["workType"],
            "about": facets["domain"],
            "keywords": facets["keywords"],
            "workType": facets["workType"],
            "domain": facets["domain"],
            "package": facets["package"],
            "artifact": "SubagentRun" if kind == "subagent" else "Conversation",
            "project": project,
            "sourcePath": str(p),
            "userTurns": turns["userTurns"],
            "assistantTurns": turns["assistantTurns"],
            "isAccessibleForFree": False,
            "license": "https://schema.org/AllRightsReserved",
            "usageInfo": "Private owner archive. Future paid download; not listed yet.",
        }
        if parent:
            item["parentTrace"] = f"{BASE}/{ROOT}/traces/{facets['package']}/{parent}.jsonld"
        item["offers"] = offer_for(item)
        items.append(item)
    items.sort(key=lambda x: x["dateModified"], reverse=True)
    return items


def turtle_catalog(packages: list[dict], counts: dict) -> str:
    lines = [
        "@prefix dct: <http://purl.org/dc/terms/> .",
        "@prefix dcat: <http://www.w3.org/ns/dcat#> .",
        "@prefix schema: <https://schema.org/> .",
        f"@prefix oid: <{NS}> .",
        "",
        f"<{BASE}/{ROOT}/catalog.jsonld> a dcat:Catalog, schema:DataCatalog ;",
        '  dct:title "Mike Holborn agent records" ;',
        f"  dct:creator <{WEBID}> ;",
        f"  oid:traceCount {counts['traces']} ;",
        f"  oid:packageCount {len(packages)} .",
        "",
    ]
    for pkg in packages:
        lines += [
            f"<{pkg['@id']}> a dcat:Dataset, schema:Dataset ;",
            f"  dct:title {json.dumps(pkg['name'])} ;",
            f"  oid:package {json.dumps(pkg['identifier'])} ;",
            f"  oid:domain {json.dumps(pkg['domain'])} ;",
            f"  dcat:theme {json.dumps(pkg['workTypes'][0]) if pkg['workTypes'] else '\"implementation\"'} ;",
            f"  schema:offerCount {len(pkg.get('hasPart') or [])} .",
            "",
        ]
    return "\n".join(lines)


PRIVATE_ACL = """@prefix acl: <http://www.w3.org/ns/auth/acl#> .

<#owner> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://www.w3.org/ns/auth/acl#Authorization> .
<#owner> <http://www.w3.org/ns/auth/acl#agent> <WEBID> .
<#owner> <http://www.w3.org/ns/auth/acl#accessTo> <BASE/ROOT/> .
<#owner> <http://www.w3.org/ns/auth/acl#default> <BASE/ROOT/> .
<#owner> <http://www.w3.org/ns/auth/acl#mode> <http://www.w3.org/ns/auth/acl#Read> .
<#owner> <http://www.w3.org/ns/auth/acl#mode> <http://www.w3.org/ns/auth/acl#Write> .
<#owner> <http://www.w3.org/ns/auth/acl#mode> <http://www.w3.org/ns/auth/acl#Append> .
<#owner> <http://www.w3.org/ns/auth/acl#mode> <http://www.w3.org/ns/auth/acl#Control> .
""".replace("WEBID", WEBID).replace("BASE/ROOT", f"{BASE}/{ROOT}")


def main() -> None:
    if not PASSWORD:
        raise SystemExit("set OPENID_PASSWORD")
    print("login")
    token = login()
    print("classify traces")
    traces = collect_traces()
    by_pkg = defaultdict(list)
    for t in traces:
        by_pkg[t["package"]].append(t)

    packages = []
    for slug, members in sorted(by_pkg.items(), key=lambda kv: -len(kv[1])):
        meta = PACKAGE_META[slug]
        works = sorted({m["workType"] for m in members})
        domains = sorted({m["domain"] for m in members})
        list_price = sum(int(m["offers"]["price"]) for m in members)
        pkg = {
            "@context": CONTEXT,
            "@id": f"{BASE}/{ROOT}/packages/{slug}.jsonld",
            "@type": ["Dataset", "dcat:Dataset", "oid:RecordPackage"],
            "identifier": slug,
            "name": meta["name"],
            "description": meta["description"],
            "audience": meta["audience"],
            "creator": {"@id": WEBID},
            "domain": domains[0] if len(domains) == 1 else domains,
            "workTypes": works,
            "keywords": sorted({k for m in members for k in m.get("keywords", [])})[:16],
            "numberOfItems": len(members),
            "hasPart": [{"@id": m["@id"], "name": m["name"]} for m in members[:80]],
            "isAccessibleForFree": False,
            "license": "https://schema.org/AllRightsReserved",
            "offers": {
                "@type": "Offer",
                "name": f"Download package: {meta['name']}",
                "availability": "https://schema.org/PreOrder",
                "priceCurrency": "USD",
                "price": list_price,
                "seller": {"@id": WEBID},
            },
        }
        packages.append(pkg)

    catalog = {
        "@context": CONTEXT,
        "@id": f"{BASE}/{ROOT}/catalog.jsonld",
        "@type": ["DataCatalog", "dcat:Catalog", "oid:RecordCatalog"],
        "name": "Mike Holborn agent records",
        "description": "Private JSON-LD archive of Grok Bot and Cursor traces, packaged for later paid download.",
        "creator": {"@id": WEBID, "name": "Mike Holborn"},
        "dateModified": iso(time.time()),
        "isAccessibleForFree": False,
        "license": "https://schema.org/AllRightsReserved",
        "dataset": [{"@id": p["@id"], "name": p["name"], "numberOfItems": p["numberOfItems"]} for p in packages],
        "counts": {
            "traces": len(traces),
            "packages": len(packages),
            "byWorkType": dict(Counter(t["workType"] for t in traces)),
            "byDomain": dict(Counter(t["domain"] for t in traces)),
            "byPackage": dict(Counter(t["package"] for t in traces)),
            "byArtifact": dict(Counter(t["artifact"] for t in traces)),
            "listPriceUSD": sum(int(t["offers"]["price"]) for t in traces),
        },
    }

    print(f"{len(traces)} traces, {len(packages)} packages")
    for c in [f"{ROOT}/", f"{ROOT}/traces/", f"{ROOT}/packages/", f"{ROOT}/grokbot/", f"{ROOT}/cursor/",
              *[f"{ROOT}/traces/{slug}/" for slug in by_pkg]]:
        put_container(token, c)
    put(token, f"{ROOT}/.acl", PRIVATE_ACL.encode(), "text/turtle")
    put_jsonld(token, f"{ROOT}/context.jsonld", {"@context": CONTEXT})
    put_jsonld(token, f"{ROOT}/catalog.jsonld", catalog)
    put_jsonld(token, f"{ROOT}/catalog.json", catalog)
    put(token, f"{ROOT}/catalog.ttl", turtle_catalog(packages, catalog["counts"]).encode(), "text/turtle")
    put_jsonld(token, f"{ROOT}/cursor/transcripts.jsonld", {"@context": CONTEXT, "@graph": traces})
    put_jsonld(token, f"{ROOT}/cursor/transcripts.json", {"@context": CONTEXT, "@graph": traces})

    for pkg in packages:
        put_jsonld(token, f"{ROOT}/packages/{pkg['identifier']}.jsonld", pkg)

    for i, t in enumerate(traces, 1):
        put_jsonld(token, f"{ROOT}/traces/{t['package']}/{t['identifier']}.jsonld", t)
        if i % 80 == 0:
            print(f"  {i}/{len(traces)}")

    grok_settings = Path("/Users/mikeholborn/.grokbot/settings.json")
    grok = redact_obj(json.loads(grok_settings.read_text())) if grok_settings.exists() else {}
    put_jsonld(token, f"{ROOT}/grokbot/settings.jsonld", {
        "@context": CONTEXT,
        "@id": f"{BASE}/{ROOT}/grokbot/settings.jsonld",
        "@type": ["DigitalDocument", "oid:AgentSettings"],
        "name": "Grok Bot settings",
        "creator": {"@id": WEBID},
        "about": "agent-identity",
        "workType": "operations",
        "package": "openid-agent-identity",
        "json": grok,
    })

    readme = f"""# Records catalog

JSON-LD / RDF archive on `{BASE}/{ROOT}/`.

- Catalog: `catalog.jsonld` and `catalog.ttl`
- Packages (buyable later): `packages/{{slug}}.jsonld`
- Traces: `traces/{{package}}/{{id}}.jsonld`
- Graph: `cursor/transcripts.jsonld`

Work types: {', '.join(sorted(catalog['counts']['byWorkType']))}
Domains: {', '.join(sorted(catalog['counts']['byDomain']))}
Not for sale yet. Offers are schema.org placeholders. Owner-only ACL.
"""
    put(token, f"{ROOT}/README.md", readme.encode(), "text/markdown")
    print(json.dumps(catalog["counts"], indent=2))


if __name__ == "__main__":
    main()
