#!/usr/bin/env python3
"""Import local AI coding-agent sessions into an OpenID pod as conversations.

Sources (auto-detected, stdlib only, Python 3.9+):
  claude   ~/.claude/projects/**/*.jsonl          (Claude Code)
  codex    ~/.codex/sessions/**/*.jsonl           (OpenAI Codex CLI / Desktop)

Each session becomes one conversation at {handle}/conversations/spark/{id}.json
via POST /conversations, so it shows up in /app next to Gemini Spark saves.
IDs are deterministic ("claude-<sessionId>", "codex-<sessionId>"), so re-running
updates in place instead of duplicating.

  BASE_URL=http://localhost:4000 OPENID_HANDLE=mike OPENID_PASSWORD=... \
    python3 scripts/import-sessions.py --limit 20
"""
from __future__ import annotations

import argparse
import getpass
import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Iterator, Optional

SECRET_RE = re.compile(
    r"(eyJ[A-Za-z0-9_\-]{20,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,})"
    r"|(sk-(?:ant-)?[A-Za-z0-9_\-]{20,})|(xai-[A-Za-z0-9]{20,})|(gh[pousr]_[A-Za-z0-9]{30,})"
    r"|(AKIA[0-9A-Z]{16})|(xox[abpr]-[A-Za-z0-9\-]{10,})"
    r"|((?:Bearer|token|secret|password|api[_-]?key)\s*[:=]\s*\S{8,})",
    re.I,
)
# Harness-injected blocks that are noise in a transcript view.
NOISE_BLOCK_RE = re.compile(
    r"<(system-reminder|environment_context|permissions instructions|user_instructions"
    r"|local-command-caveat|local-command-stdout)>.*?</\1>",
    re.S,
)
MAX_MSG_CHARS = 20_000
MAX_BODY_BYTES = 24 << 20  # server truncates LDP bodies at 32 MiB


def redact(s: str) -> str:
    return SECRET_RE.sub("[REDACTED]", s)


def clean(text: str) -> str:
    text = NOISE_BLOCK_RE.sub("", text).strip()
    if len(text) > MAX_MSG_CHARS:
        text = text[:MAX_MSG_CHARS] + f"\n\n[... truncated {len(text) - MAX_MSG_CHARS} chars]"
    return redact(text)


def safe_id(s: str) -> str:
    return re.sub(r"[^A-Za-z0-9-]", "-", s)[:120]


def read_jsonl(p: Path) -> Iterator[dict]:
    with p.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                yield json.loads(line)
            except json.JSONDecodeError:
                continue


# ---------------------------------------------------------------- Claude Code
def _claude_text(content, tools: bool) -> str:
    if isinstance(content, str):
        return content
    parts = []
    for b in content or []:
        t = b.get("type")
        if t == "text":
            parts.append(b.get("text", ""))
        elif tools and t == "tool_use":
            inp = b.get("input") or {}
            hint = inp.get("command") or inp.get("file_path") or inp.get("pattern") or inp.get("description") or ""
            parts.append(f"`[tool: {b.get('name')}] {str(hint)[:300]}`")
    return "\n\n".join(p for p in parts if p)


def parse_claude(p: Path, tools: bool) -> Optional[dict]:
    msgs, sid, cwd = [], p.stem, ""
    for o in read_jsonl(p):
        if o.get("type") not in ("user", "assistant") or o.get("isMeta") or o.get("isSidechain"):
            continue
        sid = o.get("sessionId") or sid
        cwd = cwd or o.get("cwd", "")
        m = o.get("message") or {}
        text = clean(_claude_text(m.get("content"), tools))
        if text:
            msgs.append({"role": m.get("role", o["type"]), "text": text, "timestamp": o.get("timestamp")})
    if not msgs:
        return None
    return {"id": f"claude-{safe_id(sid)}", "source": "claude-code", "cwd": cwd, "messages": msgs, "path": p}


# ---------------------------------------------------------------------- Codex
def parse_codex(p: Path, tools: bool) -> Optional[dict]:
    msgs, sid, cwd = [], p.stem, ""
    for o in read_jsonl(p):
        pl = o.get("payload") or {}
        if o.get("type") == "session_meta":
            if isinstance(pl.get("source"), dict) and "subagent" in pl["source"]:
                return None  # guardian/reviewer sub-threads; parent session has the real chat
            sid, cwd = pl.get("id", sid), pl.get("cwd", "")
            continue
        if o.get("type") != "response_item":
            continue
        if pl.get("type") == "message" and pl.get("role") in ("user", "assistant"):
            text = "\n\n".join(c.get("text", "") for c in pl.get("content") or [] if c.get("text"))
            text = clean(text)
            if text:
                msgs.append({"role": pl["role"], "text": text, "timestamp": o.get("timestamp")})
        elif tools and pl.get("type") in ("function_call", "custom_tool_call", "local_shell_call"):
            args = str(pl.get("arguments") or pl.get("input") or pl.get("action") or "")[:300]
            msgs.append({"role": "assistant", "text": redact(f"`[tool: {pl.get('name', pl['type'])}] {args}`"),
                         "timestamp": o.get("timestamp")})
    if not any(m["role"] == "user" for m in msgs):
        return None
    return {"id": f"codex-{safe_id(sid)}", "source": "codex", "cwd": cwd, "messages": msgs, "path": p}


SOURCES = {
    "claude": (Path.home() / ".claude/projects", parse_claude),
    "codex": (Path.home() / ".codex/sessions", parse_codex),
}


def title_for(s: dict) -> str:
    first = next((m["text"] for m in s["messages"] if m["role"] == "user"), "")
    first = re.sub(r"<[^>]+>", " ", first)
    first = " ".join(first.split())[:80] or "Untitled session"
    proj = Path(s["cwd"]).name if s["cwd"] else ""
    return f"[{s['source']}{' · ' + proj if proj else ''}] {first}"


# ----------------------------------------------------------------------- HTTP
def request(base: str, path: str, body: dict, token: str = "") -> dict:
    data = json.dumps(body).encode()
    req = urllib.request.Request(base + path, data=data, method="POST",
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read() or b"{}")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--source", choices=sorted(SOURCES), action="append", help="repeatable; default all")
    ap.add_argument("--limit", type=int, default=0, help="newest N sessions per source (0 = all)")
    ap.add_argument("--since", help="only files modified on/after YYYY-MM-DD")
    ap.add_argument("--tools", action="store_true", help="include one-line tool-call summaries")
    ap.add_argument("--dry-run", action="store_true", help="parse and print, do not upload")
    a = ap.parse_args()

    base = os.environ.get("BASE_URL", "http://localhost:4000").rstrip("/")
    handle = os.environ.get("OPENID_HANDLE", "")
    since = None
    if a.since:
        from datetime import datetime
        since = datetime.strptime(a.since, "%Y-%m-%d").timestamp()

    token = ""
    if not a.dry_run:
        handle = handle or input("OpenID handle: ")
        pw = os.environ.get("OPENID_PASSWORD") or getpass.getpass(f"Password for {handle}: ")
        token = request(base, "/idp/login", {"handle": handle, "password": pw})["token"]

    ok = skipped = failed = 0
    seen: set = set()  # resumed sessions span several files; newest file wins
    for name in a.source or sorted(SOURCES):
        root, parser = SOURCES[name]
        if not root.exists():
            print(f"{name}: {root} not found, skipping")
            continue
        files = sorted(root.rglob("*.jsonl"), key=lambda p: p.stat().st_mtime, reverse=True)
        if since:
            files = [f for f in files if f.stat().st_mtime >= since]
        if a.limit:
            files = files[: a.limit]
        print(f"{name}: {len(files)} session files")
        for f in files:
            s = parser(f, a.tools)
            if not s or s["id"] in seen:
                skipped += 1
                continue
            seen.add(s["id"])
            body = {"id": s["id"], "title": title_for(s), "source": s["source"],
                    "source_url": "file://" + str(f), "messages": s["messages"]}
            while len(json.dumps(body)) > MAX_BODY_BYTES and len(body["messages"]) > 2:
                body["messages"] = body["messages"][len(body["messages"]) // 4:]  # keep the tail
            if a.dry_run:
                print(f"  {s['id']}  {len(body['messages']):4d} msgs  {body['title']}")
                ok += 1
                continue
            try:
                request(base, "/conversations", body, token)
                ok += 1
                print(f"  ✓ {s['id']}  {len(body['messages'])} msgs")
            except urllib.error.HTTPError as e:
                failed += 1
                print(f"  ✗ {s['id']}  HTTP {e.code}: {e.read()[:200]!r}", file=sys.stderr)
    print(f"\nimported {ok}, skipped {skipped} (empty, subagent or older duplicate), failed {failed}")
    if not a.dry_run and ok:
        print(f"View: {base}/app  (sign in as {handle})")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
