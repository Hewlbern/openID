"""Run against a disposable local pod when OPENID_TEST_SERVER is supplied."""
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import unittest
import urllib.error
import urllib.request
import uuid

from test_knowledge import k


@unittest.skipUnless(os.environ.get("OPENID_TEST_SERVER"), "set OPENID_TEST_SERVER to a built cmd/server binary")
class KnowledgePodTests(unittest.TestCase):
    def test_upload_is_readable_only_by_its_owner_on_the_real_pod(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with socket.socket() as probe:
                probe.bind(("127.0.0.1", 0))
                port = probe.getsockname()[1]
            base = "http://127.0.0.1:" + str(port)
            env = dict(os.environ, SOLID_BASE_URL=base, SOLID_TOKEN_SECRET=uuid.uuid4().hex,
                       AUDIT_BATCH_INTERVAL="24h", IPFS_API="")
            process = subprocess.Popen([os.environ["OPENID_TEST_SERVER"], "-port", str(port),
                                        "-storage", str(root / "pod"), "-base-url", base],
                                       env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                for _ in range(100):
                    if process.poll() is not None:
                        self.fail("test pod exited before startup")
                    try:
                        urllib.request.urlopen(base + "/health", timeout=0.2).close()
                        break
                    except (OSError, urllib.error.URLError):
                        time.sleep(0.05)
                else:
                    self.fail("test pod did not start")

                def register(handle):
                    request = urllib.request.Request(base + "/idp/register", method="POST",
                        data=json.dumps({"handle": handle, "password": "LocalTest-Password-123!"}).encode(),
                        headers={"Content-Type": "application/json"})
                    with urllib.request.urlopen(request, timeout=10) as response:
                        return json.load(response)

                alice, bob = register("alice"), register("bob")
                raw = b"# Private procedure\nUse the stop button.\n"
                source = root / "manual.md"
                source.write_bytes(raw)
                bundle = root / "bundle"
                k.import_asset(source, bundle, alice["webId"], base + "/alice/knowledge/manual", kind="markdown")
                destination = k.upload_bundle(bundle, base + "/alice/", alice["token"])
                for name in ("source.bin", "manifest.jsonld", "manifest.ttl", "bundle.json"):
                    owner_request = urllib.request.Request(destination + name,
                        headers={"Authorization": "Bearer " + alice["token"]})
                    with urllib.request.urlopen(owner_request, timeout=10) as response:
                        self.assertEqual(hashlib.sha256(response.read()).digest(),
                                         hashlib.sha256((bundle / name).read_bytes()).digest())
                    for headers in ({}, {"Authorization": "Bearer " + bob["token"]}):
                        with self.assertRaises(urllib.error.HTTPError) as denied:
                            urllib.request.urlopen(urllib.request.Request(destination + name, headers=headers), timeout=10)
                        self.assertIn(denied.exception.code, (401, 403))
            finally:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
