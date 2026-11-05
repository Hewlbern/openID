import copy
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

from rdflib import Graph, Literal, RDF, URIRef, XSD
from rdflib.compare import isomorphic

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("knowledge", ROOT / "scripts/knowledge.py")
k = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(k)
OWNER = "https://alice.example/profile#me"
ASSET = "https://alice.example/knowledge/manual"


class KnowledgeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "manual.md"
        self.raw = '# Manual\n\nUse the "stop" button. Café.\n'.encode()
        self.source.write_bytes(self.raw)
        self.bundle = self.root / "asset"
        self.index = k.import_asset(self.source, self.bundle, OWNER, ASSET, kind="markdown")

    def read_graph(self, bundle=None):
        return Graph().parse(bundle or self.bundle / "manifest.ttl", format="turtle")

    def query_input(self):
        return {
            "id": "https://alice.example/runs/1", "query": "How do I stop?",
            "answer": "Use the stop button.", "model": "example-model@version-1",
            "corpusVersion": "https://alice.example/corpora/maintenance/v1",
            "retrievalConfig": {"method": "keyword", "limit": 3},
            "evidence": [{"version": self.index["record"], "sha256": hashlib.sha256(self.raw).hexdigest(),
                          "stages": ["retrieved", "context", "cited"], "rank": 1,
                          "score": 0.8, "selector": {"start": 10, "end": 32}}],
            "evaluations": [{"metric": "https://alice.example/metrics/correctness",
                             "value": 1, "evaluator": OWNER,
                             "method": "Human review against the maintenance manual"}],
        }

    def write_query(self, data):
        source = self.root / "query.json"
        source.write_text(json.dumps(data))
        return k.record_query(source, self.root / "query", OWNER)

    def test_original_bytes_versioning_and_equivalent_graphs(self):
        self.assertEqual((self.bundle / "source.bin").read_bytes(), self.raw)
        self.assertTrue(self.index["record"].endswith(hashlib.sha256(self.raw).hexdigest()))
        turtle = self.read_graph()
        linked = Graph().parse(self.bundle / "manifest.jsonld", format="json-ld")
        self.assertTrue(isomorphic(turtle, linked))
        record = URIRef(self.index["record"])
        self.assertIn((record, k.EX.controller, URIRef(OWNER)), turtle)
        self.assertIn((record, k.EX.access, Literal("private")), turtle)
        self.assertIn((record, k.EX.rightsStatus, Literal("unspecified")), turtle)
        self.assertIn((record, k.DCT.isVersionOf, URIRef(ASSET)), turtle)
        self.assertEqual(k.verify_bundle(self.bundle), self.index)
        if __import__("os").name == "posix":
            self.assertEqual(self.bundle.stat().st_mode & 0o777, 0o700)
            self.assertEqual((self.bundle / "source.bin").stat().st_mode & 0o777, 0o600)
        self.source.write_bytes(b"new revision")
        updated = k.import_asset(self.source, self.root / "revision", OWNER, ASSET)
        self.assertNotEqual(updated["record"], self.index["record"])
        self.assertEqual((self.bundle / "source.bin").read_bytes(), self.raw)

    def test_no_silent_overwrite(self):
        with self.assertRaisesRegex(ValueError, "already exists"):
            k.import_asset(self.source, self.bundle, OWNER, ASSET)

    def test_cli_import_and_verify(self):
        destination = self.root / "cli-bundle"
        imported = subprocess.run([sys.executable, str(ROOT / "scripts/knowledge.py"),
                                   "import", str(self.source), "--output", str(destination),
                                   "--webid", OWNER, "--asset-id", ASSET, "--kind", "markdown"],
                                  check=True, capture_output=True, text=True)
        self.assertEqual(json.loads(imported.stdout)["record"], self.index["record"])
        verified = subprocess.run([sys.executable, str(ROOT / "scripts/knowledge.py"),
                                   "verify", str(destination)], check=True, capture_output=True, text=True)
        self.assertEqual(json.loads(verified.stdout)["integrity"], "valid")
        self.assertEqual(json.loads(verified.stdout)["authenticity"], "not-verified")

    def test_health_derivative_keeps_claim_and_lineage_private(self):
        output = self.root / "health"
        index = k.import_asset(self.source, output, OWNER, ASSET, kind="health",
                               sensitivity="health", deidentification="pseudonymised",
                               derived_from=["https://alice.example/private/original"],
                               license_uri="https://alice.example/licenses/research")
        g = self.read_graph(output / "manifest.ttl")
        record = URIRef(index["record"])
        self.assertIn((record, k.PROV.wasDerivedFrom, URIRef("https://alice.example/private/original")), g)
        self.assertIn((record, k.EX.deidentificationClaim, Literal("pseudonymised")), g)
        self.assertIn((record, k.EX.access, Literal("private")), g)

    def test_query_records_stages_selectors_and_attributed_measurements(self):
        data = self.query_input()
        self.write_query(data)
        output = self.root / "query"
        k.verify_bundle(output)
        g = self.read_graph(output / "manifest.ttl")
        run, version = URIRef(data["id"]), URIRef(self.index["record"])
        self.assertIn((run, k.PROV.used, version), g)
        evidence = URIRef(str(run) + "#evidence-0")
        for stage in ("retrieved", "context", "cited"):
            self.assertIn((run, k.EX[stage], evidence), g)
        self.assertIn((URIRef(str(evidence) + "-selector"), k.OA.start,
                       Literal(10, datatype=XSD.nonNegativeInteger)), g)
        measurement = URIRef(str(run) + "#evaluation-0")
        self.assertIn((measurement, k.DQV.computedOn, run), g)
        self.assertIn((measurement, k.PROV.wasAttributedTo, URIRef(OWNER)), g)
        self.assertNotIn((version, k.DQV.hasQualityMeasurement, measurement), g)

    def test_retrieved_is_not_automatically_used(self):
        data = self.query_input()
        data["evidence"][0]["stages"] = ["retrieved"]
        self.write_query(data)
        g = self.read_graph(self.root / "query/manifest.ttl")
        self.assertNotIn((URIRef(data["id"]), k.PROV.used, URIRef(self.index["record"])), g)

    def test_invalid_evidence_and_measurements_do_not_create_bundle(self):
        mutations = [
            lambda d: d["evaluations"][0].update(value=float("nan")),
            lambda d: d["evaluations"][0].pop("evaluator"),
            lambda d: d["evidence"][0].update(stages=["cited"]),
            lambda d: d["evidence"][0].update(rank=True),
            lambda d: d["evidence"][0].update(sha256="wrong"),
            lambda d: d["evidence"][0].update(selector={"start": 10, "end": 5}),
            lambda d: d.update(unknown="silently lost field"),
        ]
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                data = copy.deepcopy(self.query_input())
                mutate(data)
                with self.assertRaises(ValueError):
                    self.write_query(data)
                self.assertFalse((self.root / "query").exists())

    def update_checksum(self, name):
        index = json.loads((self.bundle / "bundle.json").read_text())
        index["files"][name]["sha256"] = k.digest((self.bundle / name).read_bytes())
        (self.bundle / "bundle.json").write_text(json.dumps(index))

    def test_detects_modified_payload(self):
        (self.bundle / "source.bin").write_bytes(b"modified")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            k.verify_bundle(self.bundle)

    def test_detects_different_graphs_even_with_updated_checksums(self):
        g = self.read_graph()
        g.add((URIRef(self.index["record"]), k.EX.access, Literal("public")))
        g.serialize(self.bundle / "manifest.ttl", format="turtle")
        self.update_checksum("manifest.ttl")
        with self.assertRaisesRegex(ValueError, "different graphs"):
            k.verify_bundle(self.bundle)

    def test_parser_errors_do_not_repeat_private_content(self):
        (self.bundle / "manifest.ttl").write_text("PRIVATE-HEALTH-CONTENT is not Turtle")
        self.update_checksum("manifest.ttl")
        with self.assertRaises(ValueError) as error:
            k.verify_bundle(self.bundle)
        self.assertEqual(str(error.exception), "invalid RDF metadata")

    def test_rejects_traversal_and_remote_jsonld_contexts(self):
        index_path = self.bundle / "bundle.json"
        original = index_path.read_bytes()
        index = json.loads(original)
        index["files"]["../outside"] = index["files"].pop("source.bin")
        index_path.write_text(json.dumps(index))
        with self.assertRaisesRegex(ValueError, "expected files"):
            k.verify_bundle(self.bundle)
        index_path.write_bytes(original)
        doc = json.loads((self.bundle / "manifest.jsonld").read_text())
        doc["@context"] = "https://never-fetch.example/context"
        (self.bundle / "manifest.jsonld").write_text(json.dumps(doc))
        self.update_checksum("manifest.jsonld")
        with self.assertRaisesRegex(ValueError, "inline"):
            k.verify_bundle(self.bundle)

    def test_rejects_symlinked_payload(self):
        (self.bundle / "source.bin").unlink()
        (self.bundle / "source.bin").symlink_to(self.source)
        with self.assertRaisesRegex(ValueError, "symlinks"):
            k.verify_bundle(self.bundle)

    def server(self, fail_acl=False, redirect=False, corrupt=False):
        requests = []
        resources = {}
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def do_HEAD(self):
                requests.append(("HEAD", self.path, dict(self.headers), b""))
                self.send_response(302 if redirect else 200)
                if redirect:
                    self.send_header("Location", "/redirect-target/")
                self.end_headers()

            def do_PUT(self):
                body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
                requests.append(("PUT", self.path, dict(self.headers), body))
                resources[self.path] = body
                self.send_response(403 if fail_acl and self.path.endswith("/.acl") else 201)
                self.end_headers()

            def do_GET(self):
                requests.append(("GET", self.path, dict(self.headers), b""))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b"truncated" if corrupt and self.path.endswith("source.bin") else resources[self.path])
        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        return "http://127.0.0.1:" + str(server.server_port) + "/alice/", requests

    def test_upload_sets_owner_acl_before_content_and_commits_index_last(self):
        container, requests = self.server()
        url = k.upload_bundle(self.bundle, container, "test-token")
        self.assertTrue(url.startswith(container + "asset-"))
        self.assertEqual([r[0] for r in requests], ["HEAD", "PUT", "PUT", "GET", "PUT", "GET", "PUT", "GET", "PUT", "GET", "PUT"])
        self.assertTrue(requests[2][1].endswith("/.acl"))
        acl = Graph().parse(data=requests[2][3], format="turtle")
        self.assertEqual(set(acl.objects(None, k.ACL.agent)), {URIRef(OWNER)})
        self.assertFalse(list(acl.objects(None, k.ACL.agentClass)))
        self.assertEqual(set(acl.objects(None, k.ACL.default)), {URIRef(url)})
        self.assertEqual(requests[4][3], self.raw)
        self.assertTrue(requests[-1][1].endswith("/bundle.json"))
        for method, _, headers, _ in requests:
            self.assertEqual(headers["Authorization"], "Bearer test-token")
            if method == "PUT":
                self.assertEqual(headers["If-None-Match"], "*")

    def test_acl_failure_never_uploads_content(self):
        container, requests = self.server(fail_acl=True)
        with self.assertRaisesRegex(ValueError, "HTTP 403"):
            k.upload_bundle(self.bundle, container, "test-token")
        self.assertEqual(len(requests), 3)
        self.assertTrue(requests[-1][1].endswith("/.acl"))

    def test_read_back_mismatch_never_commits_the_bundle(self):
        container, requests = self.server(corrupt=True)
        with self.assertRaisesRegex(ValueError, "read-back checksum"):
            k.upload_bundle(self.bundle, container, "test-token")
        self.assertFalse(any(r[1].endswith("bundle.json") for r in requests))

    def test_oversized_input_is_rejected_before_writing(self):
        with self.source.open("wb") as stream:
            stream.truncate(k.MAX_BYTES + 1)
        with self.assertRaisesRegex(ValueError, "16 MiB"):
            k.import_asset(self.source, self.root / "oversized", OWNER, ASSET)
        self.assertFalse((self.root / "oversized").exists())

    def test_rejects_redirects_and_insecure_remote_upload(self):
        container, requests = self.server(redirect=True)
        with self.assertRaisesRegex(ValueError, "redirects"):
            k.upload_bundle(self.bundle, container, "test-token")
        self.assertEqual(len(requests), 1)
        with self.assertRaisesRegex(ValueError, "HTTPS"):
            k.upload_bundle(self.bundle, "http://remote.example/alice/", "test-token")


if __name__ == "__main__":
    unittest.main()
