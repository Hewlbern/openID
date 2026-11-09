# Knowledge assets and query evidence

OpenID's knowledge collector preserves original files and describes how exact
versions were used in AI tasks. Markdown, documents, chat exports, health
datasets, and robot recordings are all knowledge assets. A query run links to
asset versions and records supplied retrieval, context, citation, and evaluation
evidence. It does not generate an answer or infer that a source caused success.

This first implementation is a local Python CLI with optional upload through
the existing Solid LDP API. It needs Python 3.9+ and RDFLib:

```sh
python3 -m venv .venv-knowledge
.venv-knowledge/bin/python -m pip install -r scripts/requirements-knowledge.txt
```

## Preserve a knowledge asset

```sh
.venv-knowledge/bin/python scripts/knowledge.py import ./manual.md \
  --output ./bundles/manual-v1 \
  --webid https://alice.example/profile/card#me \
  --asset-id https://alice.example/knowledge/manual \
  --kind markdown
```

The command prints the exact version identifier. Reuse `--asset-id` when
importing revisions. Version identifiers contain the SHA-256 of the original
file bytes. New bytes produce a new version; existing output directories are
never overwritten. Identical bytes can have multiple metadata snapshots (for
example, a later rights declaration); the version identifies the source bytes,
not a cryptographically signed metadata snapshot.
`recordedAt` describes collection time, not the source's original creation time.

Each bundle contains:

| File | Purpose |
| --- | --- |
| `source.bin` | Exact original bytes, including Markdown formatting or binary data |
| `manifest.jsonld` | RDF graph serialized as JSON-LD with an inline context |
| `manifest.ttl` | The same RDF graph serialized as Turtle |
| `bundle.json` | Record identity, controller WebID, media types and file checksums |

The original name is used as a default title; pass `--title` when that name is
sensitive. Local bundles have owner-only filesystem permissions on POSIX.

Options include `--kind document|markdown|chat|health|robot|dataset`,
`--media-type`, `--source-uri`, `--license` (a URI), and repeatable
`--derived-from` version URIs. A source URI is metadata; the collector does not
fetch it. Omitted rights are explicitly `unspecified`. A controller WebID is a
claim about control of this bundle, not proof of copyright or source authorship.

For a prepared derivative, use `--sensitivity personal|health` and
`--deidentification redacted|pseudonymised|anonymised`. These are explicit claims
about a transformation already performed. This tool does not remove secrets,
anonymise people, verify consent, or publish data. All bundles remain private,
including those marked anonymised. Links to source records, titles, prompts,
embeddings, and measurements can themselves be sensitive.

Files are limited to 16 MiB each. The current Go LDP handler truncates incoming
bodies at 32 MiB; the collector keeps below that limit and verifies uploaded
bytes. Streaming, chunked uploads and large MCAP recordings require a subsequent
change. No source is silently truncated.

## Record a query and its evidence

Prepare a JSON file such as the following. Replace the version and digest with
the values from your imported bundle; these placeholders are not executable:

```json
{
  "id": "https://alice.example/runs/stop-procedure-1",
  "query": "How should this machine be stopped?",
  "answer": "Use the stop button.",
  "model": "example-model@version-1",
  "corpusVersion": "https://alice.example/corpora/maintenance/v1",
  "retrievalConfig": {"method": "keyword", "limit": 3},
  "evidence": [{
    "version": "EXACT_VERSION_URI",
    "sha256": "64_CHARACTER_LOWERCASE_SOURCE_DIGEST",
    "stages": ["retrieved", "context", "cited"],
    "rank": 1,
    "score": 0.8,
    "selector": {"start": 10, "end": 32}
  }],
  "evaluations": [{
    "metric": "https://alice.example/metrics/correctness",
    "value": 1,
    "evaluator": "https://alice.example/profile/card#me",
    "method": "Human review against the maintenance manual"
  }]
}
```

```sh
.venv-knowledge/bin/python scripts/knowledge.py record-query ./query.json \
  --output ./bundles/query-1 \
  --webid https://alice.example/profile/card#me
```

`id`, `query`, `model`, `corpusVersion`, `evidence`, and `evaluations` are
required. Empty evidence/evaluation arrays are permitted and mean none was
recorded. `answer` and `retrievalConfig` are optional. Unknown fields are rejected
to avoid silently discarding observations. Configuration is preserved as an RDF
JSON literal. Query input bytes are retained exactly in `source.bin`.

An evidence item requires an absolute version URI, a SHA-256 digest, and stages:

- `retrieved`: a retrieval system returned the source.
- `context`: source content was actually included in the model input.
- `cited`: the output cited it; this requires `context` too.

Context can also come from an attached document without retrieval. Only context
inclusion creates `prov:used`; ranking alone never does. The optional selector
uses Web Annotation text positions: zero-based Unicode character offsets,
exclusive end, into the referenced UTF-8 source text. Offsets and referenced
digests are caller-supplied claims; this CLI does not fetch sources to validate
them. Non-text sources should omit text selectors.

Each evaluation requires a metric URI, finite numeric value, evaluator URI,
and method description. An optional `baseline` URI identifies a comparison run.
Metric definitions should specify units, scale, direction and rubric. Measurements
apply to the query run, not universally to its source documents. Use separate
run IDs for different models, corpus versions, or controlled comparisons.

## Vocabulary and serialization

Both exports are generated from a single RDFLib graph, avoiding hand-written
JSON-to-Turtle mappings. Verification compares the graphs, including datatypes.

- PROV-O: source versions, derivation, actors and query activities.
- Dublin Core / DCAT: version relationships, rights and file distributions.
- Web Annotation: references to exact text passages.
- DQV: attributed, query-specific quality measurements (a W3C Working Group Note).
- `https://github.com/Hewlbern/openID/ns/knowledge#`: this experimental project's
  terms, including `KnowledgeAsset`, `AssetVersion`, `QueryRun`, `EvidenceUse`,
  `controller`, `sha256`, `retrieved`, `context`, `cited`, `deidentificationClaim`.

The project namespace identifies the profile; it is not yet a hosted ontology.
`bundlePath` maps a distribution to its file in the bundle. A pod upload retains
the logical record IDs and exposes the files at the returned bundle URL.

RDF describes the knowledge and evidence. Original Markdown, FHIR JSON, chat
JSONL or MCAP bytes remain their native format. FHIR normalization, ontology
mapping, RDF reasoning, embedding generation, SHACL validation and telemetry
collection are not implemented by this collector.

## Verify and upload privately

```sh
.venv-knowledge/bin/python scripts/knowledge.py verify ./bundles/manual-v1

# Set OPENID_TOKEN securely in the environment to a pod bearer token.
# Spark connect tokens are restricted to conversations and cannot upload here.
.venv-knowledge/bin/python scripts/knowledge.py upload ./bundles/manual-v1 \
  --container https://pod.example/alice/
```

Use an existing container you can write. Upload creates a fresh child container,
sets an owner-only ACL, reads the ACL back, then uploads and verifies each data
file. `bundle.json` is uploaded last as the completion marker. ACL failure,
checksum mismatch or HTTP failure stops the upload; a failed attempt may leave
an empty container or a partial private bundle without a completion marker.
Retries currently create a new child. No existing ACL or bundle is overwritten.

The client refuses redirects and remote plaintext HTTP. Only loopback HTTP is
allowed for development. It uses a bearer token, not the server's custom agent
signature or incomplete DPoP implementation. Access control still depends on
the pod enforcing WAC; this feature does not remediate the server authentication
issues or protect data from the server operator. Do not treat it as approval to
upload sensitive health data to an unreviewed deployment.

Verification checks file checksums, source-to-manifest binding and RDF graph
equivalence. It does **not** prove authorship, consent, truth, complete capture,
or resistance to an attacker replacing the whole unsigned bundle. It reports
`authenticity: not-verified`. Remote JSON-LD contexts and symlinked files are
refused during verification.

## Tests

```sh
.venv-knowledge/bin/python -m unittest discover -s test/python -v
go build -o /tmp/openid-test-server ./cmd/server
OPENID_TEST_SERVER=/tmp/openid-test-server \
  .venv-knowledge/bin/python -m unittest discover -s test/python -v
```

The integration test starts a disposable loopback pod, registers two synthetic
users, uploads a bundle and checks that its owner can read every file while
anonymous requests and the other user cannot. CI includes this test.

## Subsequent PRs

1. Fix WebID/key binding, DPoP validation, durable audit recovery and OTS proof
   verification; add client-signed manifests with key history.
2. Add resumable source adapters, version catalog/search, passage extraction,
   SHACL validation, and runtime query capture via OpenTelemetry.
3. Correlate robot episodes, MCAP, ROS execution traces and query decisions with
   explicit clock domains, safety interventions and outcomes.
4. Publish separately reviewed derivatives and signed dataset offers; federate
   catalogs and add XEC and Lightning payment adapters. Payment does not establish
   data rights, anonymity or delivery. Public identities must not expose private
   subjects through provenance links.
