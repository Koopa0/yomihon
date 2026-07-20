# Semantic embedding provider protocol (2026-07-13)

This is the H4 protocol-step output. It pins only the Gemini provider client;
provider choice remains behind the consumer-side embedder interfaces.

## Research Report

### Questions

- What is the stable successor model and input limit?
- What exact asymmetric retrieval prompts does it require?
- What request cardinality, dimension, and response handling preserve numerical
  comparability?
- Which error classes are provider-owned, and which one is sufficiently clear
  to identify a yomihon-formed malformed request?

### Pre-Research Hypothesis

The successor would accept one text `Content`, support a selected output
dimension, and normalize truncated vectors. Query and document prompts would
need explicit versioning; SDK retries would need to be disabled. Model ID,
task-type support, aggregation, and fault ownership required verification.

### Findings

1. [Gemini Embedding 2 model](https://ai.google.dev/gemini-api/docs/models/gemini-embedding-2)
   identifies stable model `gemini-embedding-2`, an 8,192-token text input
   limit, dimensions 128–3,072, and recommended sizes 768/1,536/3,072.
2. [Gemini embeddings guide](https://ai.google.dev/gemini-api/docs/embeddings)
   requires asymmetric retrieval text to use query structure
   `task: search result | query: {content}` and document structure
   `title: {title} | text: {content}`. It explicitly says Embedding 2 does not
   accept the older task-type field, aggregates multiple inputs in one request,
   and automatically normalizes truncated dimensions.
3. [Embeddings REST reference](https://ai.google.dev/api/embeddings) defines the
   `embedContent` endpoint, one `content`, `EmbedContentConfig`, and response
   field `embedding.values`.
   Its public v1beta discovery schema (revision 20260712) includes
   `EmbedContentConfig.autoTruncate`, but the current official Go, Python, and
   JavaScript SDK converters reject that setting on the Gemini Developer API
   as Enterprise-only. Those SDKs route Developer API text through the batch
   endpoint while this client uses direct single `embedContent`, so the
   conflict does not prove either direct-wire behavior offline.
4. [Gemini troubleshooting](https://ai.google.dev/gemini-api/docs/troubleshooting)
   attributes `INVALID_ARGUMENT` to a malformed request body,
   `PERMISSION_DENIED` to credential permissions, `RESOURCE_EXHAUSTED` to rate
   limits, and `INTERNAL`/`UNAVAILABLE` to provider-side failure. Google
   [AIP-193](https://google.aip.dev/193) fixes the JSON error envelope, and the
   [canonical status definitions](https://grpc.io/docs/guides/status-codes/)
   identify `UNAUTHENTICATED` as invalid authentication credentials.

### Post-Research Conclusion

**YES, research changed the hypothesis.** Dimension and normalization were
confirmed, but task intent is prompt text rather than a request field, and
multiple inputs aggregate. The existing contextual chunk string therefore had
to be restructured into the documented document form, and batching is not an
equivalent optimization.

### Recommendation for Planner

Use the documented raw REST endpoint with an injected `http.RoundTripper`, not
the SDK. This keeps request bytes, redirects, retries, error bodies, and the
single-send observable under yomihon's control. The alternative SDK is easier
to call but adds retry/observability behavior that H5 would have to prove
disabled and kept disabled across SDK upgrades.

## Pinned protocol epoch

- Model: `gemini-embedding-2`.
- Endpoint: `POST https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-2:embedContent`.
- Authentication: key only in `x-goog-api-key`; never in the URL or body.
- Request cardinality: one HTTP request, one `content`, one text part, one
  embedding. No `batchEmbedContents`, no multiple content parts, and no
  provider aggregation.
- Query prompt: exact UTF-8 `task: search result | query: {raw query}`.
- Document prompt: exact UTF-8 `title: {context title} | text: {chunk body}`.
  Context title is the note title followed by the heading ancestry with ` › `;
  a continuation appends ` — part n/m`. If the local title is empty it is
  `none`. The chunk cap includes this complete provider prompt.
- Request JSON: one compact object with `content.parts[0].text` and
  `embedContentConfig.autoTruncate=false` plus
  `embedContentConfig.outputDimensionality={dimension}`. No `taskType`, title
  config, model body field, unknown field, or trailing JSON value.
  The `autoTruncate` field is therefore **live-acceptance gated**, not claimed
  certified from contradictory offline documentation. It stays present because
  silently removing it would abandon H3's fail-loud oversize contract.
- Dimension: **1,536**, selected by H9's live paired comparison. Both 1,536
  and 3,072 produced the same 40/40 required-positive rank-1 result and the
  same 40/40 contrast-below-positive result; 3,072 supplied no measured
  retrieval benefit while doubling vector payload and exact-scan work. The
  selected dimension is explicit in every request and remains part of
  identity; it is never an implicit provider default. Small dimensions in unit
  fixtures remain offline test inputs, not dispatchable configuration.
- Response handling: decode exactly one `embedding.values`; require exactly
  `dimension` finite float32-compatible values; preserve returned values as
  float32 and do no client-side normalization. Optional usage metadata has no
  current consumer and is ignored rather than collapsing absent and zero into
  a misleading metric; response bodies are never retained.
- Retry/redirect: the HTTP client makes one `RoundTrip` per method call and
  follows no redirect. Its dedicated production transport sets `Proxy=nil`, so
  `HTTP_PROXY`/`HTTPS_PROXY` cannot silently create a second egress route; a
  proxy would require a new decision and an injected, observable boundary.
  Query methods and interactive document calls are never retried. Explicit
  build document calls are owned by H4's scheduler: provider configuration and
  construction precede any ledger change, then the scheduler durably reserves
  one send slot before invoking the chunk-send capability. Any later abort
  consumes that slot even if `RoundTrip` was never reached. A successful vector
  and attempt-row clear commit together. Only 429 creates another client call
  inside the same action; a valid `Retry-After` overrides the 1s/4s/9s/16s
  fallback, and a wait over 30s is persisted for a later action. Each storage
  generation grants at most five slots per pending chunk; additional batches
  require the explicit D60 renewal action and are not hidden transport behavior. The
  provider exposes no stable structured oversized-input discriminator, so the
  scheduler never parses error-message text or splits and retries a rejected
  body after the response; local H3 cap splitting happens before submission.
- Response body bound: 8 MiB. A larger success or error body is
  `embedder-failed`; it is neither logged nor forwarded.

## Total error mapping

The client parses only the bounded Google error envelope's numeric `code` and
canonical `status`; it discards `message` and `details`, which may echo request
bytes. HTTP/status disagreement is unclassifiable and therefore provider-owned.

| Observable terminal | Local outcome | Ownership |
|---|---|---|
| Transport returns no HTTP response | `embedder-unreachable` | provider/network |
| HTTP 429 + `RESOURCE_EXHAUSTED` | `rate-limited` | provider |
| HTTP 401 + `UNAUTHENTICATED` | `embedder-rejected` | credential/provider |
| HTTP 403 + `PERMISSION_DENIED` | `embedder-rejected` | credential/provider |
| HTTP 400 + `INVALID_ARGUMENT` | confirmed malformed request / internal error | yomihon |
| Matching documented 5xx status, any other documented status, malformed error envelope, HTTP/status disagreement, or unknown status | `embedder-failed` | provider by Koopa's uncertainty ruling |
| 2xx response missing one finite vector of the exact dimension, trailing JSON, or exceeding the bound | `embedder-failed` | provider/unclassifiable |
| Local request construction fails before `RoundTrip` | internal error | yomihon; zero egress |

No classification parses provider `message` text. The table's catch-all is
deliberately exit-3 territory; only the exact, agreeing malformed-body class is
an internal error. `rate-limited` classifies that provider response; it does not
classify scheduler authorization. On the last slot, H4 preserves a prerequisite
that must be repaired first: malformed request remains internal and credential
rejection remains `embedder-rejected`. Only 429, unreachable, or unknown/provider
failure becomes `attempt-budget-exhausted`, because another send for that same
target requires explicit renewal. Five slots upper-bound HTTP requests but may
include pre-transport aborts, so they are never reported as five observed sends.

## Live protocol acceptance evidence

The gate ran on 2026-07-16 through the exact direct REST client under D57's
fixed-synthetic certification exception:

1. the byte-frozen short request above succeeds at each candidate dimension;
2. a deterministic ASCII fixture first measured by the provider's count-tokens
   endpoint above 8,192 tokens is rejected by the same direct endpoint with an
   agreeing `400/INVALID_ARGUMENT`, rather than returning a successfully
   truncated vector.

Both 1,536 and 3,072 returned the exact requested length and provider-normalized
finite vectors. The count-tokens-confirmed over-limit request returned the
ruled malformed-request terminal instead of a vector. The paired fixed
synthetic recording then produced 40 query rows and 32 corpus-chunk rows at
each dimension and passed structural validation; H9 records the relevance
comparison and the 1,536 selection.

The probes sent no vault or user query bytes. Any future rejection of
`autoTruncate`, successful over-limit truncation, or changed response shape
reopens the protocol and H3 policy before another corpus build; no
implementation silently drops the field or accepts truncation.
