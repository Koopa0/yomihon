# Judge face — implementation plan (spec §5)

> Status: **built and merged in full** (all four stages: rules, formats, CLI dispatch, coverage/exists, gating — PR #11–#14); the four cron consumers switched over 2026-07-05. The remaining prerequisite before the retirement declaration is the differential campaign (§13). The judge face is
> `kurodo check` / `exists` / `coverage` — a Go rewrite of kura's CLI. Its soul
> is **byte-compatibility with kura**, a different discipline from the search
> face's determinism: the retirement gate (D11) is a byte-for-byte match of
> kura's JSONL and its conformance snapshots, plus switching the four real
> pipelines over. Every "contract" below is quoted from the actual kura source
> (`/Users/koopa/rust/github.com/koopa0/kura`) — verified, not paraphrased.

## 1. Goal and the retirement gate (D11)

Reproduce kura's `check` / `exists` / `coverage` output **byte-for-byte**, so the
four pipelines and the manual gates can switch from `kura` to `kurodo` without
noticing, and kura can finally be retired. Until this is met, kura does not change
a line — it stands as the golden reference.

## 2. Deployment shape — DB-free, stateless, fresh walk (§0.1)

This is the hard constraint. The judge face is **not** part of the serve /
in-memory-snapshot world. Each `kurodo check` invocation is a stateless process
that walks the vault fresh and exits — kura's exact shape (no server, no daemon,
no database, zero background state). §0.1: "`check` / `exists` / `coverage` are
stateless file scans that never touch the DB." If the judge face needed the
running server or any persistent store, that would be a deployability regression
and a covert tightening of the gate — forbidden.

It **may reuse** the pure library packages (`internal/graph` resolution,
`internal/vault` walk/parse, `internal/schema` contract loading) — those are the
same primitives kura uses — but it adds its own rule engine, its own byte-exact
`Finding` serialization, and the CLI subcommands.

## 3. The byte-exact contracts (the acceptance basis)

### 3a. The `Finding` JSONL line

Field order (serde declaration order; Go: struct field order), with kura's
omit rules:

| # | field | Go type | present |
|---|---|---|---|
| 1 | `rule_id` | string | always |
| 2 | `severity` | string | always — **lowercase** `"info"`/`"warn"`/`"error"` |
| 3 | `path` | string | always |
| 4 | `line` | `*int` | omit when nil (`omitempty` on a pointer) |
| 5 | `field` | `*string` | omit when nil |
| 6 | `message` | string | always |
| 7 | `evidence` | string | always |
| 8 | `suggested_action` | string | always |
| 9 | `source_rule` | string | always |
| 10 | `target` | `*string` | omit when nil |
| 11 | `resolved_to` | `*string` | omit when nil (in practice only `map.disk_unlisted` sets it) |
| 12 | `collision_members` | `[]string` | omit when empty |
| 13 | `fingerprint` | string | always |

Byte rules (all verified against `tests/snapshots/conformance__jsonl_output.snap`):
- **Compact** JSON, no spaces; one object per line, `\n` after each.
- **CJK is raw UTF-8**, never `\uXXXX`; `->` in messages is ASCII `0x2d 0x3e`.
- The always-present strings (`message` etc.) must serialize even when empty →
  **no `omitempty`** on them. Only `line`/`field`/`target`/`resolved_to` (pointers)
  and `collision_members` (slice) get `omitempty`. This exactly mirrors kura's
  `skip_serializing_if = "Option::is_none"` / `"Vec::is_empty"`.

**The Go byte-compat gotcha (must pin):** `encoding/json.Marshal` HTML-escapes
`<`, `>`, `&`; serde_json does **not**. So the encoder must be
`enc := json.NewEncoder(w); enc.SetEscapeHTML(false)`, one `enc.Encode(finding)`
per finding — `Encode` writes compact JSON + a trailing `\n`, matching kura's
`to_jsonl()` exactly. Using `json.Marshal` would silently escape `->`-adjacent
or `&` bytes and break the diff.

The four golden lines (from the snapshot) are the primary acceptance target;
they are reproduced in the test fixtures verbatim.

### 3b. The fingerprint (FNV-1a 64-bit)

```
offset = 0xcbf29ce484222325 ; prime = 0x00000100000001b3
h = offset
for part in [rule_id, path, target]:
    for b in utf8(part): h = (h XOR b) * prime   (mul wraps mod 2^64)
    h = (h XOR 0x1f) * prime                      # separator after EACH part (3x)
return fmt.Sprintf("%016x", h)                     # lowercase, zero-padded 16
```

Per-rule feed of `path`/`target` is load-bearing (a mismatch changes the hash):
- `link.broken` / `link.title_not_alias`: `(rule, note.path, link.target)`.
- `provenance.unresolved`: `("provenance.unresolved", note.path, rawValue)` where
  rawValue keeps the `[[...]]` (e.g. `[[Missing]]`).
- `collision.alias`: `("collision.alias", "", normalizedAlias)` — **empty path**,
  alias is the normalized (trim/NFC/lower) key.
- `map.disk_mismatch`: `("map.disk_mismatch", syllabus.path, link.target)`.
- `map.disk_unlisted`: `("map.disk_unlisted", lesson.path, syllabus.path)`.
- `link.broken.path`: `("link.broken.path", note.path, ref.target)`.
- `schema.*`: `(rule_id, note.path, field + "\x1f" + value)` — the `target`
  argument itself contains an **embedded `0x1f`** (so there are effectively two
  separators around the field name; reproduce the exact string).

### 3c. Sort — `(path, line, rule_id)`

`line` absent = `0` (so line-less findings sort before line 1 of the same path).
String comparison is **bytewise on UTF-8** (Go `<` on strings and `slices.SortFunc`
with `strings.Compare` match Rust's `&str` `Ord`). Stable sort. Sort before emit.

### 3d. `coverage` — compact JSON object (not the pretty snapshot)

Struct: `{total_concepts int, domains []{domain, concepts, mounted, pending_mount,
orphan}, pending_mount []string, orphans []string, unrouted []{path, note_type,
expected_route}}` — snake_case, every field always present, declaration order.
Sorted: domains by domain, pending_mount/orphans lexicographic, unrouted by path.
**The CLI emits compact `json.Marshal`-equivalent + `\n`** — the `.snap` file is
pretty-printed by insta and is NOT the on-wire form; match the compact CLI bytes.
No cron consumes coverage, so this is parity-for-completeness, not load-bearing.

### 3e. `exists` — the dedup oracle

Matches (normalized query vs): filename **stem**, full filename (if different),
`title`, every `alias`, `title_en` — deliberately **wider than the resolver**
(it matches title/title_en, which resolution never does). Output (compact JSON
+ `\n`): `{"query":..., "matches":[{"path":..., "field":"filename|title|alias|title_en", "value":...}]}`,
sorted by `(path, field)`. Exit **0** if any match, **1** if none.

## 4. The 15 rules

From `RULE_IDS` (kura `src/lib.rs`): `link.title_not_alias`, `link.broken`,
`collision.alias`, `provenance.unresolved`, `map.disk_mismatch`, `map.disk_unlisted`,
`link.broken.path`, `schema.enum`, `schema.required`, `schema.unknown_key`,
`schema.slug`, `schema.domain_folder`, `schema.legacy_tag`, `schema.provenance`,
`schema.frontmatter`.

- **Graph-consuming (6):** the six link/map/provenance rules share the wikilink
  resolver — kurodo's `internal/graph` already reproduces kura's `graph.rs`
  semantics (trim→NFC→lower normalize, four key forms + aliases, title never a
  key, ambiguous-not-guessed). `link.title_not_alias` additionally needs a
  title→note index; `provenance.unresolved` a slug index; `collision.alias` an
  aliases-only, case-insensitive+NFC index.
- **Filesystem (1):** `link.broken.path` — `stat` a `[text](path.md)` / backticked
  `path.md`; in-root miss = Warn, escapes-root = Info (not stat'd).
- **Schema (8):** all Error severity, fixed strings (`evidence` =
  "frontmatter validated against vault-schema.toml", `suggested_action` = "fix the
  frontmatter to match the schema", `source_rule` = "vault-schema.toml"); scoped to
  `[scan].knowledge_dirs`, skipping `[scan].skip_basenames`; no-frontmatter is
  legal; a present-but-unparseable block → exactly one `schema.frontmatter`.

Severity/gating detail (Info vs Warn for planned forward-refs on `link.broken` and
`map.disk_mismatch`) is in the fact-gathering notes and will be pinned per rule
during the build.

## 5. CLI surface, `--deny`, exit codes

- `check [PATHS...] [--all] [--deny <val>]... [--baseline <file>]`, `coverage`,
  `exists <name>`; global `--root` (default cwd), `--format json|human|md`.
- Positional PATHS only **filter output**; the graph is always built whole-tree.
- `--deny <val>`: a severity keyword (`error`/`warn`/`info`) OR a rule id, repeatable.
  Severity → the **minimum** denied severity is the threshold; any finding ≥ it
  gates. Rule id → that rule gates **only at Warn+** (an Info finding never gates
  via its rule id). An unknown `--deny` token → exit 2.
- **Exit codes: 0** clean (no deny-level finding; with no `--deny`, always 0),
  **1** gate-hit, **2** tool-error (bad root, unknown flag, unreadable baseline,
  unparseable schema) — printed to **stderr** as `kura: <err>` … (see §8 on the
  `kura:` prefix decision).
- Format resolution: explicit `--format` wins; else **json when stdout is piped,
  human when a TTY**. `md` is check-only (coverage/exists fall back to human).

## 6. Scan boundary

- Walk the whole tree; hidden entries (`.obsidian`, `.git`, `.trash`) skipped;
  **gitignore is NOT honored** (Obsidian ignores git; a gitignored attachment is a
  live link target). `.md` → notes, everything else → linkable resources. Sorted by
  path. kurodo's `internal/vault` walk must match this (confirm it skips dotfiles
  and does not consult `.gitignore`).
- Default finding scope drops a finding only if **every** path it touches (citing
  path + collision members) starts with `System/`; `--all` disables this. No
  separate `Diagrams/`/`Views/` filter at this layer.
- Schema-check scope is separate: only notes whose first path segment ∈
  `[scan].knowledge_dirs`, skipping `[scan].skip_basenames`.

## 7. NFC

Reuse `graph.NormalizeNFC` (already exported) + the shared `trim → NFC → lower`
(Unicode lowercase, not ASCII) — kurodo's `graph.normalize` already matches kura's
`graph.rs::normalize` byte-for-byte. Walk-time path normalization must also be NFC
(kura NFC-normalizes every relative path); `internal/vault.List` already does this
(D-note: verified earlier). A mismatch here breaks both resolution and fingerprints
(collision.alias / schema.* feed normalized text into the hash).

## 8. Package layout, and two naming/output decisions

- New package `internal/judge` (or `internal/check`): the `Finding` type + its
  byte-exact serializer, the fingerprint, the 15 rules, and `check`/`exists`/
  `coverage`. `cmd/kurodo` gains the `check`/`exists`/`coverage` subcommands
  (dispatch only; the existing `serve` is untouched).
- **Open decision — the error prefix.** kura prints tool errors as `kura: <err>`.
  For byte-exactness of stderr, do we emit `kura:` (a lie — it's kurodo) or
  `kurodo:`? stderr is not consumed by any cron (they read exit codes and stdout),
  so `kurodo:` is safe and honest. Proposed: `kurodo:`. Confirm.
- **Open decision — coverage/exists pretty vs compact.** The CLI is compact; I'll
  match the compact CLI bytes (the pretty `.snap` is insta's, not on-wire).

## 9. Testing — byte-compat is proven two ways

1. **Golden conformance, byte-exact.** Port kura's `conformance.rs` fixtures (the
   3-note setup) and assert `kurodo check` stdout equals the 4 golden JSONL lines
   **byte-for-byte** (quoted in §3a), and the coverage/exists compact forms.
2. **Real-vault diff vs the reference (the strongest test).** A test that skips
   unless `KURODO_REFERENCE_BIN` names an executable (the only way any test may
   locate the reference binary — no hardcoded paths in the repo; the binding
   lives in the shell environment outside it), runs both engines with
   `check --root ~/obsidian --format json`, sorts nothing (both already
   sorted), and asserts the two byte streams are identical. Same for
   `coverage`, and for `exists <name>` on a few names. This is the gate that
   actually proves retirement-readiness. (As merged: `sandwich_test.go` and
   the `referenceTool` helper in `schema_test.go`.)
3. **The four crons' load-bearing bytes** (from the fact-gather): exit code under
   `--deny error` (0 vs nonzero); the compact JSONL containing literal
   `link.broken` and `"severity":"warn"` (grinder greps these); the `--format md`
   report body + the `--format human` first line
   (`"{N} findings: {E} error, {W} warn, {H} hidden (...)"`). A test asserts each.
4. Per-rule unit tests (all stdlib + go-cmp; no Docker — the judge face has no DB).

## 10. Load-bearing vs parity-for-completeness

**Load-bearing (a cron breaks if wrong):** exit codes under `--deny error`; the
compact JSONL bytes for `link.broken` + `"severity":"warn"`; the `--format md`
report body and `--format human` summary first line. **Parity-for-completeness
(no live consumer, but required for the retirement gate):** the full JSONL of all
15 rules, `coverage`, `exists`, `--baseline`, `--format` niceties. Both must be
byte-exact for retirement, but if the build is staged, the load-bearing set is the
switchover blocker; the rest can land before the gate is declared met.

## 11. Scope and open decisions

- **Build all 15 rules + coverage + exists** in this face (the retirement gate and
  the real-vault diff need the whole surface). `--baseline` (delta gate) has **zero
  live consumers** (verified) — implement for byte-compat, not as a hot path.
- Open decisions for sign-off: (1) error prefix `kurodo:` vs `kura:` (§8 — I
  propose `kurodo:`); (2) package name `internal/judge` vs `internal/check`
  (I propose `judge` — it is the judge face, and it holds three commands, not just
  check); (3) confirm the real-vault diff-vs-kura test as the primary acceptance
  gate (it needs `~/.cargo/bin/kura` present and a quiescent vault at test time).
- Out of scope: the serve/snapshot world (untouched), the write face (wall 1),
  search, export, any frontend.

## 12. Divergence register (the complete list; the retirement gate = byte-compat except exactly these)

Every deliberate or latent departure from the reference bytes, each verified
against the real binary and each with its guard stated honestly. Anything not
on this list that diffs is a bug.

1. **stderr error prefix `kurodo:`** (deliberate; ruled in §8). No consumer
   reads stderr; exit codes and stdout are the contract.
2. **`--format md` preamble, two lines** (`tool: kurodo`, `# kurodo check`;
   deliberate, Koopa 2026-07-05). Golden hand-pinned; the real-vault sandwich
   compares the report body and skips the preamble. **Switchover checklist
   item**: confirm the md-consuming pipeline does not match on the old tool
   name before flipping it.
3. **Path references inside `%%…%%` comments are not checked** (deliberate,
   Koopa 2026-07-05; the reference checks wikilinks' comment scope but not
   path refs — an inconsistency in the reference, corrected here). Fixture +
   dedicated test pin the intent; the real vault has no such case, so the
   sandwich also holds.
4. **`coverage` folds an explicit empty `domain: ""` into `(none)`** (latent;
   the reference emits a separate `""` group). Unreachable: the schema
   requires a non-empty domain, and the real vault has none. Guard: real —
   if such a note appears, the sandwich's coverage subtest fails loudly.
5. **`exists` with an empty query skips empty `title`/`title_en` values; an
   empty `alias` entry still matches** (latent; the reference matches all
   empty-string metadata). Unreachable in practice (no empty titles/aliases in
   the vault; no consumer issues an empty query). Guard: **weak, stated
   honestly** — the sandwich queries fixed names and would not notice; the
   divergence can only manifest on a degenerate call nothing makes. If exists
   is ever touched again, unify the empty-value rule for self-consistency.
6. **An unreadable contract file is a tool error (exit 2, loud)** (latent; the
   reference silently skips all schema rules and exits 0). kurodo's behavior
   is strictly safer for a gating tool — a broken contract should never look
   like a clean vault. Guard: environmental; if the contract file is ever
   unreadable during the sandwich, the run fails visibly.
7. **Malformed flag and filter input is a tool error (exit 2, loud)**
   (deliberate, 2026-07-05, three cases from review: `--root=` with an empty
   value, `--all=` with a non-true value, and a path filter that normalizes to
   an empty prefix — the reference silently falls back to cwd / default /
   match-nothing respectively, so malformed input can read as a clean scan).
   Well-formed input is unaffected — a nonexistent-but-valid path filter still
   matches nothing with exit 0, same as the reference (verified both sides).
   Unreachable from the four pipelines (their invocations are fixed strings);
   guard: the property only manifests on a malformed call nothing makes, and
   the dedicated flag tests pin it.
8. **Findings whose resolution touches the private daily journal are dropped
   from check output** (deliberate, Koopa 2026-07-05; a finding is dropped when
   a path it touches — its citing path, a collision member, or the note a link
   resolved to — begins with Diary/, unconditionally, the full unfiltered set
   included; and a link resolving to a journal note's title is dropped at its
   source, so the journal path never reaches the finding's evidence text. The
   link's own target text is the citing author's words and is deliberately not
   counted. The reference reports a journal note's findings like any other's;
   this fail-closed drop is the divergence). The journal is still scanned, so
   its links resolve for other notes. Fixture + dedicated test pin the intent
   (the drop holds under the full set and across the citing, collision,
   resolved-to, and title-owner channels; a broken link outside the journal is
   still reported); the real vault currently emits no journal-touching finding,
   so the sandwich also holds.
9. **The existence oracle never reports a note in the private daily journal**
   (deliberate, Koopa 2026-07-05; exists drops every match whose path is under
   Diary/, so a dedup query cannot learn a journal note's name, path, or alias.
   The oracle is a read by an agent — an egress path the journal must stay out
   of — and the reference matches journal notes like any other). The real
   vault's fixed exists queries name no journal note, so the sandwich holds; a
   dedicated test pins the drop and that a public note still matches. Coverage
   also drops every concept or routable note under Diary/, so a journal note
   mistyped as a concept never reaches that report either; a fixture with such a
   note and a test pin it.
10. **Journal content exerts no influence on egress verdicts** (deliberate,
   D42, ruled 2026-07-06 under delegation): the reference counts a journal
   note's outgoing edges toward coverage's referenced/mounted sets, and its
   planned-name lists toward broken-link severity; this engine excludes
   journal sources from both, so a public concept's mount state and a public
   broken link's severity never encode journal content. A public link naming
   a journal title remains the author's own words (entry 8) and its
   resolution is unchanged. Guard: dedicated fixtures pin both exclusions (a
   journal-only mount edge leaves the concept orphaned here, mounted on the
   reference; a journal-only planned name leaves the public link warn here,
   info there). The same excluded mount set also feeds the routed/unrouted
   classification, so a routable note mounted only by a journal map counts
   as unrouted here and routed on the reference — the same divergence seen
   through a third window, not a separate mechanism; the differential
   generator does not construct that case (the normalizer set stays closed)
   and the real-vault sandwich guards it. The sandwich was run at
   implementation time and held: the real journal currently exerts no such
   influence on any surface.

## 13. The differential campaign (the declaration's final prerequisite)

The register (§12) freezes the known differences; the campaign hunts unknown
ones, on inputs nobody wrote by hand. The harness lives in `internal/judge`,
env-gated like every reference-coupled test (`KURODO_REFERENCE_BIN`; never in
CI): a seeded, schema-aware vault generator biased toward the rule surfaces; a
manifest recording the intentionally divergence-prone constructs each vault
plants (comment-wrapped path refs, journal-crossing links, empty domains), so
register filtering is precise rather than heuristic; one named normalizer per
register entry (unit-tested; entries avoided by construction are documented as
such); and a runner that byte-diffs both engines' `check`, `coverage`, and
`exists` output after normalization, preserving vault + seed + first diff hunk
on any mismatch.

The register mapping is fixed in advance so the filter set is closed: entries
1, 5, 6, and 7 are **avoided by construction** (the runner compares stdout and
exit codes only, never issues an empty `exists` query, never makes the
contract file unreadable, and only uses fixed well-formed invocations — each
documented as such next to the normalizer table); entries 2 and 4 get
**mechanical normalizers** (skip the two-line md preamble; fold the
reference's empty-string coverage domain into its `(none)` form); entries 3,
8, 9, and 10 get **manifest-driven filters** (drop reference findings at
manifest-known comment-wrapped path-ref sites; drop reference findings/matches
whose manifest-known resolution touches the journal through any of the four
channels; and re-derive the reference's coverage mount state and broken-link
severity at manifest-known journal-influence sites, since this engine
excludes journal sources from both computations).

**Completion bar — the retirement declaration cites this section when all five
hold:**

1. At least 5,000 generated vaults cumulatively, across at least three
   independent runs with distinct base seeds. (As first written this demanded
   separate calendar days; Koopa waived the day boundary on 2026-07-07 — the
   third run's independence came from a fresh seed base and a main that had
   absorbed the intervening hardening work, which is the variance the clause
   was after. Run dates are still recorded per item 5.)
2. Zero unexplained byte differences after §12 normalization.
3. The rule-reach self-check passes in every run: all fifteen rules and both
   frontmatter failure classes — a note with no frontmatter block at all, and
   a note whose block does not parse — exercised at least once.
4. Every divergence the campaign finds is adjudicated before its run counts,
   and adjudication is not the implementer's call: the campaign pauses, the
   finding goes to Koopa (walls and schema are always his; the guide session
   triages), and only then is a kurodo defect fixed or a reference defect
   recorded as a new §12 entry with a pinning fixture. A run containing an
   unadjudicated diff does not count toward item 1.
5. The declaration records the totals: rounds, base seeds, run dates, and the
   aggregated rule-reach distribution.

After the declaration, the scaffolding dies and the contract stays: delete the
conformance, sandwich, and differential tests; keep every golden and pinning
fixture; remove the reference-binary shell export; clean the wrapper backups;
delete the reference binaries (D40).
