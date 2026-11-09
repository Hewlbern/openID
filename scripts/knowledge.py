#!/usr/bin/env python3
"""Create and privately upload portable knowledge and query-evidence bundles."""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import mimetypes
import os
from pathlib import Path
import re
import shutil
import sys
import tempfile
from datetime import datetime, timezone
import urllib.error
import urllib.parse
import urllib.request
import uuid

from rdflib import Graph, Literal, Namespace, RDF, URIRef, XSD
from rdflib.compare import isomorphic

EX = Namespace("https://github.com/Hewlbern/openID/ns/knowledge#")
PROV = Namespace("http://www.w3.org/ns/prov#")
DCAT = Namespace("http://www.w3.org/ns/dcat#")
DCT = Namespace("http://purl.org/dc/terms/")
DQV = Namespace("http://www.w3.org/ns/dqv#")
OA = Namespace("http://www.w3.org/ns/oa#")
ACL = Namespace("http://www.w3.org/ns/auth/acl#")
# The current pod handler truncates bodies at 32 MiB. Leave room for metadata
# and bound local memory until a separate streaming/chunked importer exists.
MAX_BYTES = 16 * 1024 * 1024
KINDS = ("document", "markdown", "chat", "health", "robot", "dataset")


def iri(value):
    if not isinstance(value, str) or not re.match(r"^(https?://|urn:)", value):
        raise ValueError("identifiers must be absolute HTTP(S) or URN identifiers")
    if re.search(r'[\s<>"{}|\\^`]', value):
        raise ValueError("identifier contains unsafe characters")
    parts = urllib.parse.urlsplit(value)
    if parts.scheme in ("http", "https") and (not parts.hostname or parts.username or parts.password):
        raise ValueError("HTTP identifiers must have a host and no credentials")
    return URIRef(value)


def read_bytes(path):
    with Path(path).open("rb") as stream:
        data = stream.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise ValueError("files over 16 MiB require a streaming importer")
    return data


def digest(data):
    return hashlib.sha256(data).hexdigest()


def json_bytes(value):
    return (json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + "\n").encode("utf-8")


def graph():
    result = Graph()
    for prefix, ns in (("knowledge", EX), ("prov", PROV), ("dcat", DCAT),
                       ("dct", DCT), ("dqv", DQV), ("oa", OA)):
        result.bind(prefix, ns)
    return result


def base_graph(record, owner, kind):
    g = graph()
    g.add((record, RDF.type, EX[kind]))
    g.add((record, EX.controller, iri(owner)))
    g.add((record, EX.access, Literal("private")))
    g.add((record, EX.recordedAt, Literal(datetime.now(timezone.utc).isoformat(), datatype=XSD.dateTime)))
    return g


def write_bundle(output, g, record, owner, kind, payload, media_type):
    """Commit the index last inside a private directory; never overwrite a bundle."""
    output = Path(output)
    if output.exists():
        raise ValueError("output already exists; choose a new bundle directory")
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".knowledge-", dir=output.parent))
    try:
        files = {
            "source.bin": (payload, media_type),
            "manifest.jsonld": (g.serialize(format="json-ld", auto_compact=True).encode(), "application/ld+json"),
            "manifest.ttl": (g.serialize(format="turtle").encode(), "text/turtle"),
        }
        if any(len(content) > MAX_BYTES for content, _ in files.values()):
            raise ValueError("generated bundle files exceed the 16 MiB limit")
        index = {"schemaVersion": 1, "kind": kind, "record": str(record),
                 "ownerWebId": str(iri(owner)), "files": {}}
        for name, (content, content_type) in files.items():
            target = temporary / name
            target.write_bytes(content)
            target.chmod(0o600)
            index["files"][name] = {"sha256": digest(content), "mediaType": content_type}
        (temporary / "bundle.json").write_bytes(json_bytes(index))
        (temporary / "bundle.json").chmod(0o600)
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return index


def import_asset(source, output, owner, asset_id, kind="document", title=None,
                 license_uri=None, source_uri=None, derived_from=(),
                 deidentification="none", sensitivity="general", media_type=None):
    if kind not in KINDS:
        raise ValueError("unsupported asset kind")
    if deidentification not in ("none", "redacted", "pseudonymised", "anonymised"):
        raise ValueError("unsupported deidentification claim")
    if sensitivity not in ("general", "personal", "health"):
        raise ValueError("unsupported sensitivity")
    asset = iri(asset_id)
    if urllib.parse.urlsplit(str(asset)).query or urllib.parse.urlsplit(str(asset)).fragment:
        raise ValueError("asset-id must not contain a query or fragment")
    payload = read_bytes(source)
    content_hash = digest(payload)
    version = iri(str(asset).rstrip("/") + "/versions/" + content_hash)
    g = base_graph(version, owner, "AssetVersion")
    g.add((asset, RDF.type, EX.KnowledgeAsset))
    g.add((version, RDF.type, PROV.Entity))
    g.add((version, DCT.isVersionOf, asset))
    g.add((version, EX.kind, Literal(kind)))
    g.add((version, DCT["title"], Literal(title or Path(source).name)))
    g.add((version, EX.sha256, Literal(content_hash)))
    g.add((version, EX.deidentificationClaim, Literal(deidentification)))
    g.add((version, EX.sensitivity, Literal(sensitivity)))
    g.add((version, EX.rightsStatus, Literal("declared" if license_uri else "unspecified")))
    if license_uri:
        g.add((version, DCT.license, iri(license_uri)))
    if source_uri:
        g.add((version, DCT.source, iri(source_uri)))
    for parent in derived_from:
        g.add((version, PROV.wasDerivedFrom, iri(parent)))
    media_type = media_type or {".md": "text/markdown", ".jsonl": "application/x-ndjson",
                                ".mcap": "application/vnd.mcap"}.get(Path(source).suffix.lower()) \
        or mimetypes.guess_type(str(source))[0] or "application/octet-stream"
    if not re.fullmatch(r"[\w.+-]+/[\w.+-]+", media_type):
        raise ValueError("media type must be a bare type/subtype without parameters")
    distribution = URIRef(str(version) + "/distribution")
    g.add((version, DCAT.distribution, distribution))
    g.add((distribution, RDF.type, DCAT.Distribution))
    g.add((distribution, DCAT.mediaType, Literal(media_type)))
    g.add((distribution, DCAT.byteSize, Literal(len(payload), datatype=XSD.nonNegativeInteger)))
    g.add((distribution, EX.bundlePath, Literal("source.bin")))
    return write_bundle(output, g, version, owner, "asset", payload, media_type)


def fields(obj, allowed, required=()):
    if not isinstance(obj, dict):
        raise ValueError("expected a JSON object")
    if set(obj) - set(allowed):
        raise ValueError("unknown fields: " + ", ".join(sorted(set(obj) - set(allowed))))
    if set(required) - set(obj):
        raise ValueError("missing fields: " + ", ".join(sorted(set(required) - set(obj))))


def text_field(value, name):
    if not isinstance(value, str) or not value.strip():
        raise ValueError(name + " must be a nonempty string")
    return Literal(value)


def number(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)) or (isinstance(value, float) and not math.isfinite(value)):
        raise ValueError("measurements must be finite numbers")
    return Literal(value)


def record_query(source, output, owner):
    payload = read_bytes(source)
    data = json.loads(payload)
    fields(data, ("id", "query", "answer", "model", "corpusVersion", "retrievalConfig", "evidence", "evaluations"),
           ("id", "query", "model", "corpusVersion", "evidence", "evaluations"))
    run = iri(data["id"])
    if urllib.parse.urlsplit(str(run)).fragment:
        raise ValueError("query run id must not have a fragment")
    g = base_graph(run, owner, "QueryRun")
    g.add((run, RDF.type, PROV.Activity))
    g.add((run, PROV.wasAssociatedWith, iri(owner)))
    for key in ("query", "model"):
        g.add((run, EX[key], text_field(data[key], key)))
    if "answer" in data:
        g.add((run, EX.answer, text_field(data["answer"], "answer")))
    g.add((run, EX.corpusVersion, iri(data["corpusVersion"])))
    g.add((run, EX.sha256, Literal(digest(payload))))
    if "retrievalConfig" in data:
        if not isinstance(data["retrievalConfig"], dict):
            raise ValueError("retrievalConfig must be an object")
        g.add((run, EX.retrievalConfig, Literal(json.dumps(data["retrievalConfig"], allow_nan=False), datatype=RDF.JSON)))
    for key in ("evidence", "evaluations"):
        if not isinstance(data[key], list):
            raise ValueError(key + " must be an array")
    for i, evidence in enumerate(data["evidence"]):
        fields(evidence, ("version", "sha256", "stages", "rank", "score", "selector"), ("version", "sha256", "stages"))
        version = iri(evidence["version"])
        if not isinstance(evidence["sha256"], str) or not re.fullmatch(r"[0-9a-f]{64}", evidence["sha256"]):
            raise ValueError("evidence sha256 must be a lowercase SHA-256 digest")
        stages = evidence["stages"]
        if not isinstance(stages, list) or not stages or any(s not in ("retrieved", "context", "cited") for s in stages):
            raise ValueError("evidence stages must contain retrieved, context, or cited")
        if "cited" in stages and "context" not in stages:
            raise ValueError("cited evidence must also declare context inclusion")
        item = URIRef(str(run) + "#evidence-" + str(i))
        g.add((item, RDF.type, EX.EvidenceUse))
        g.add((item, PROV.entity, version))
        g.add((item, EX.sha256, Literal(evidence["sha256"])))
        for stage in stages:
            g.add((run, EX[stage], item))
        if "context" in stages:
            g.add((run, PROV.used, version))
        if "rank" in evidence:
            rank = evidence["rank"]
            if isinstance(rank, bool) or not isinstance(rank, int) or rank < 1:
                raise ValueError("rank must be a positive integer")
            g.add((item, EX.rank, Literal(rank, datatype=XSD.positiveInteger)))
        if "score" in evidence:
            g.add((item, EX.retrievalScore, number(evidence["score"])))
        if "selector" in evidence:
            selector = evidence["selector"]
            fields(selector, ("start", "end"), ("start", "end"))
            start, end = selector["start"], selector["end"]
            if any(isinstance(v, bool) or not isinstance(v, int) for v in (start, end)) or not 0 <= start < end:
                raise ValueError("selector requires nonnegative start < end character offsets")
            target, selection = URIRef(str(item) + "-target"), URIRef(str(item) + "-selector")
            g.add((item, OA.hasTarget, target))
            g.add((target, RDF.type, OA.SpecificResource))
            g.add((target, OA.hasSource, version))
            g.add((target, OA.hasSelector, selection))
            g.add((selection, RDF.type, OA.TextPositionSelector))
            for key, value in (("start", start), ("end", end)):
                g.add((selection, OA[key], Literal(value, datatype=XSD.nonNegativeInteger)))
    for i, evaluation in enumerate(data["evaluations"]):
        fields(evaluation, ("metric", "value", "evaluator", "method", "baseline"), ("metric", "value", "evaluator", "method"))
        measurement = URIRef(str(run) + "#evaluation-" + str(i))
        g.add((run, DQV.hasQualityMeasurement, measurement))
        g.add((measurement, RDF.type, DQV.QualityMeasurement))
        g.add((measurement, DQV.computedOn, run))
        g.add((measurement, DQV.isMeasurementOf, iri(evaluation["metric"])))
        g.add((measurement, DQV.value, number(evaluation["value"])))
        g.add((measurement, PROV.wasAttributedTo, iri(evaluation["evaluator"])))
        g.add((measurement, EX.method, text_field(evaluation["method"], "method")))
        if "baseline" in evaluation:
            g.add((measurement, EX.baseline, iri(evaluation["baseline"])))
    return write_bundle(output, g, run, owner, "query", payload, "application/json")


def verify_bundle(directory):
    directory = Path(directory)
    if (directory / "bundle.json").is_symlink():
        raise ValueError("bundle files must not be symlinks")
    index = json.loads(read_bytes(directory / "bundle.json"))
    fields(index, ("schemaVersion", "kind", "record", "ownerWebId", "files"),
           ("schemaVersion", "kind", "record", "ownerWebId", "files"))
    if index["schemaVersion"] != 1 or index["kind"] not in ("asset", "query"):
        raise ValueError("unsupported bundle schema")
    record, owner = iri(index["record"]), iri(index["ownerWebId"])
    if not isinstance(index["files"], dict) or set(index["files"]) != {"source.bin", "manifest.jsonld", "manifest.ttl"}:
        raise ValueError("bundle must contain exactly the three expected files")
    for name, info in index["files"].items():
        fields(info, ("sha256", "mediaType"), ("sha256", "mediaType"))
        if (directory / name).is_symlink():
            raise ValueError("bundle files must not be symlinks")
        if digest(read_bytes(directory / name)) != info["sha256"]:
            raise ValueError("checksum mismatch: " + name)
        if not isinstance(info["mediaType"], str) or not re.fullmatch(r"[\w.+-]+/[\w.+-]+", info["mediaType"]):
            raise ValueError("invalid media type")
    # Refuse remote JSON-LD contexts before invoking a processor which could fetch them.
    document = json.loads(read_bytes(directory / "manifest.jsonld"))
    def check_context(value):
        if isinstance(value, dict):
            for key, child in value.items():
                if key == "@context" and not isinstance(child, dict):
                    raise ValueError("only inline JSON-LD contexts are supported")
                if key == "@import":
                    raise ValueError("remote JSON-LD context imports are not supported")
                check_context(child)
        elif isinstance(value, list):
            for child in value:
                check_context(child)
    check_context(document)
    try:
        left = Graph().parse(data=json.dumps(document), format="json-ld")
        right = Graph().parse(data=read_bytes(directory / "manifest.ttl"), format="turtle")
    except Exception:
        # Parser diagnostics can quote the source, which may contain health or
        # chat data. Keep failure output independent of the payload.
        raise ValueError("invalid RDF metadata") from None
    if not isomorphic(left, right):
        raise ValueError("JSON-LD and Turtle describe different graphs")
    if (record, EX.controller, owner) not in left or (record, EX.access, Literal("private")) not in left:
        raise ValueError("manifest ownership or access disagrees with bundle")
    if (record, EX.sha256, Literal(index["files"]["source.bin"]["sha256"])) not in left:
        raise ValueError("manifest source checksum disagrees with bundle")
    return index


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("pod redirects are refused; use the final pod URL")


def upload_bundle(directory, container, token):
    """Upload into a fresh child; establish its private ACL before any content."""
    index = verify_bundle(directory)
    iri(container)
    parts = urllib.parse.urlsplit(container)
    if parts.scheme != "https" and not (parts.scheme == "http" and parts.hostname in ("localhost", "127.0.0.1", "::1")):
        raise ValueError("uploads require HTTPS except on loopback")
    if parts.query or parts.fragment or not container.endswith("/"):
        raise ValueError("container must end in / and have no query or fragment")
    if not token or any(c.isspace() for c in token):
        raise ValueError("set OPENID_TOKEN to a pod bearer token")
    opener = urllib.request.build_opener(NoRedirect())
    destination = container + index["kind"] + "-" + str(uuid.uuid4()) + "/"

    def request(method, url, data=None, content_type=None, extra=None):
        headers = {"Authorization": "Bearer " + token}
        if content_type:
            headers["Content-Type"] = content_type
        headers.update(extra or {})
        req = urllib.request.Request(url, data=data, method=method, headers=headers)
        try:
            with opener.open(req, timeout=30) as response:
                if not 200 <= response.status < 300:
                    raise ValueError("pod returned an unexpected status")
                body = response.read(MAX_BYTES + 1)
                if len(body) > MAX_BYTES:
                    raise ValueError("pod response exceeds the 16 MiB limit")
                return body
        except urllib.error.HTTPError as error:
            # Never print response bodies, tokens, or possibly sensitive server output.
            raise ValueError("pod request failed: HTTP " + str(error.code)) from None

    request("HEAD", container)
    request("PUT", destination, b"", "text/turtle", {
        "If-None-Match": "*", "Link": '<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"'})
    acl = graph()
    authorization = URIRef(destination + ".acl#owner")
    acl.add((authorization, RDF.type, ACL.Authorization))
    acl.add((authorization, ACL.agent, iri(index["ownerWebId"])))
    acl.add((authorization, ACL.accessTo, URIRef(destination)))
    acl.add((authorization, ACL.default, URIRef(destination)))
    for mode in (ACL.Read, ACL.Write, ACL.Control):
        acl.add((authorization, ACL.mode, mode))
    request("PUT", destination + ".acl", acl.serialize(format="nt").encode(), "text/turtle", {"If-None-Match": "*"})
    acl_body = request("GET", destination + ".acl")
    try:
        stored_acl = Graph().parse(data=acl_body, format="turtle")
    except Exception:
        raise ValueError("pod returned invalid ACL metadata") from None
    if not isomorphic(acl, stored_acl):
        raise ValueError("pod did not preserve the private ACL")
    for name, info in index["files"].items():
        request("PUT", destination + name, read_bytes(Path(directory) / name), info["mediaType"], {"If-None-Match": "*"})
        if digest(request("GET", destination + name)) != info["sha256"]:
            raise ValueError("pod read-back checksum mismatch: " + name)
    request("PUT", destination + "bundle.json", json_bytes(index), "application/json", {"If-None-Match": "*"})
    return destination


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    asset = commands.add_parser("import", help="preserve a file and describe its exact version")
    asset.add_argument("source")
    asset.add_argument("--output", required=True)
    asset.add_argument("--webid", required=True)
    asset.add_argument("--asset-id", required=True)
    asset.add_argument("--kind", choices=KINDS, default="document")
    asset.add_argument("--title")
    asset.add_argument("--license", dest="license_uri")
    asset.add_argument("--source-uri")
    asset.add_argument("--derived-from", action="append", default=[])
    asset.add_argument("--deidentification", choices=("none", "redacted", "pseudonymised", "anonymised"), default="none")
    asset.add_argument("--sensitivity", choices=("general", "personal", "health"), default="general")
    asset.add_argument("--media-type")
    query = commands.add_parser("record-query", help="record supplied retrieval and evaluation evidence")
    query.add_argument("source")
    query.add_argument("--output", required=True)
    query.add_argument("--webid", required=True)
    verify = commands.add_parser("verify", help="check local integrity, not authorship or signatures")
    verify.add_argument("directory")
    upload = commands.add_parser("upload", help="upload privately using OPENID_TOKEN")
    upload.add_argument("directory")
    upload.add_argument("--container", required=True, help="existing writable pod container URL")
    args = vars(parser.parse_args(argv))
    command = args.pop("command")
    try:
        if command in ("import", "record-query"):
            args["owner"] = args.pop("webid")
            result = import_asset(**args) if command == "import" else record_query(**args)
            print(json.dumps({"record": result["record"], "output": args["output"]}))
        elif command == "verify":
            result = verify_bundle(**args)
            print(json.dumps({"integrity": "valid", "record": result["record"], "authenticity": "not-verified"}))
        else:
            print(json.dumps({"url": upload_bundle(token=os.environ.get("OPENID_TOKEN", ""), **args), "access": "private"}))
        return 0
    except (ValueError, OSError, urllib.error.URLError) as error:
        print("knowledge: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
