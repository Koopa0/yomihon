# Product (positioning, modes, and the aesthetic charter)

This document is the product lens: what yomihon is *for*, how its owner's
activities shape its surfaces, and the taste every visual decision answers
to. The functional canon (spec, plan docs) says what each face does; this
says why the whole thing deserves to exist when Obsidian is already open in
the next window.

## 1. Positioning

yomihon is **the human terminal of a human-and-agent knowledge system**. The
vault is not a folder of notes; it is a living pipeline — agents draft,
translate, examine, and propose; one human reads, decides, and directs.
Obsidian is the authoring and revision workbench. yomihon renders *the
work*: what changed, what needs a decision, what an agent claims, how
knowledge connects, how far a course of study has come. The seal — reading
a thing and ruling on it in the same breath — is the product's heart, and
everything else is built around getting the owner to that moment with less
friction and more context.

It is deliberately **not an editor**. Authoring and revising prose belongs
to Obsidian, which is excellent at it; yomihon writes exactly one
frontmatter field under a state machine (wall 1) and renders everything
else faithfully (wall 4). This division is not a limitation to apologize
for — it is the product architecture: the editor and the terminal stay
different tools with different postures, and each is better for it.

Nor is it a guesser. yomihon deterministically interprets **one
contract-bearing vault**: vault-schema.toml, maps, paths, links, status, and
git. `vault-model.md` deliberately scopes that interpretation to this vault,
not any knowledge base. The human UI and every agent-facing CLI derive their
semantics from the same vault authority even when their execution paths
differ; the diagnostics CLI's frozen byte contract is a format boundary, not
a second interpretation. Semantic and AI capabilities may suggest related
content, rankings, summaries, diagnoses, and classifications, but never own
schema truth, authoritative membership, privacy, mode existence, content
hiding, status, or a write.

## 2. Why HTML beats an "advanced Obsidian"

An Obsidian plugin lives inside someone else's document model, someone
else's sidebar, someone else's idea of a workspace. A server-rendered page
owns its whole surface: real forms with a real state machine behind them,
native dialogs and popovers, cross-document transitions, speech synthesis,
purpose-built lesson interactions (furigana, slot machines, concept
sheets), diagnostics rendered *in place*, and a typography tuned for
CJK-heavy long-form reading — all under one aesthetic, none of it fighting
a host application. The platform ladder (D41) is the discipline that keeps
this power calm: semantic HTML, then CSS, then Chromium APIs, then a little
script.

## 3. Modes (the interface adapts to the activity)

One user, four recurring postures. Surfaces should meet the posture — the
sidebar's context-first redesign was the first installment of this
principle, not the last.

| Mode | What the owner is doing | Serving it today | Coming |
|---|---|---|---|
| **Study** | Working through lessons, drilling patterns, sealing what is finished | Reading page + five interactions; syllabus wayfinding (Here, auto-opened paths); smoothness and view-transition layer; the seal | Hover layer (§11); further study UX is pain-driven |
| **Adjudicate** | Triaging what agents produced; deciding statuses in batches | Per-note status panel; the backlog number as the doorway | The cockpit (D): queue, decide-in-place, proposal inbox — the throughput face |
| **Observe** | Reading system reports, diagnostics, coverage; asking "is the vault healthy" | Sandboxed reports (E); per-note diagnostics; `check`/`coverage` CLI | In-place diagnostic cards (§11); H's graph verbs for "what references what" |
| **Reflect** | Humanities reading — book reviews, term studies; rereading the private journal | All render today (journal reading is local, never egress) | Library-map affordances ride H's relation verbs; reflection UX is pain-driven |

The rule that keeps this honest: modes are *postures of the same person*,
not user segments — no feature ships for a hypothetical user, and observed
pain reorders the roadmap (program.md holds the order).

## 4. The constitutional queue (wall-1 amendments, parked deliberately)

The write face's narrowness is the system's most load-bearing promise, so
anything that widens it is a constitutional amendment: named here, shaped
in advance, ruled only by Koopa, and only when real usage makes the case.

1. **seal-applies-patch** (already parked, roadmap §3): letting an accepted
   proposal apply its patch — the cockpit's accept loop stays out-of-band
   until this is ruled.
2. **Capture** (parked 2026-07-07, shaped by the product lens): the one
   authoring act a terminal legitimately wants is *capture* — a fleeting
   thought into `Inbox/`, a diary line into today's journal — because
   losing the thought while switching tools is real friction. If ever
   ruled in, its shape is **append-only creation**: new file, or append to
   today's journal file; never editing existing content, never a prose
   editor — and human-only in any first shape: no agent-facing capture
   write enters with it. Until the ruling, capture stays in Obsidian and
   this entry exists so the idea has a disciplined landing zone instead of
   an improvised one.

Reading the journal in yomihon needs no amendment — local rendering is not
egress (D39/D42 guard the machine-readable outputs, not the owner's own
eyes). Any future diary capture inherits that fail-closed boundary: local
only, never embedded, sent to an LLM or agent context, or exposed to an
external API.

## 5. The aesthetic charter (delegated to the guide, 2026-07-07)

The look is **the scholar's desk, not the dashboard**: yomihon serves
a sovereign's reading room. Decisions that follow from it:

- **Paper first.** Calm warm surfaces, restrained borders, generous
  measure; nothing blinks for attention. Density belongs to the sidebar
  and the future cockpit; the article column stays quiet.
- **Type is the interface.** Serif for reading (CJK-first: 明體 lineage,
  ruby as a first-class citizen, punctuation trimmed), monospace for the
  machinery (paths, statuses, diagnostics), and the two never blur. The
  smoothness inventory (ux-plan §12) is typography work, not decoration.
- **One ceremonial moment.** The seal is the only theatrical interaction —
  press-and-hold weight, a single settle pulse. Everything else moves only
  to preserve context (transitions, disclosure) or to answer (cards,
  previews). Motion is meaning; reduced-motion strips it all without loss.
- **Chrome in Traditional Chinese, content in its own language** (D28): the
  interface speaks to Koopa; the vault keeps the author's language.
- **The design bundle remains the visual source** for components; this
  charter governs what new surfaces may look like when the bundle is
  silent.
- **The owner tunes the reading, never the meaning.** A bounded set of
  reading preferences (D48) may adjust presentation — theme, type, measure,
  ruby — within this charter's house style. The default stays complete on
  its own, and no preference moves interpretation, membership, or a wall.

## 6. Product principles (the short list)

1. The seal moment is sacred: fewer steps to it, more context around it.
2. Terminal, not editor — capture may someday enter by amendment; editing
   never.
3. Human UI and agent CLI share one vault interpretation. An agent surface
   exists only when an agent is a legitimate actor; human-only actions do
   not gain agent writes for symmetry.
4. The interface adapts to the activity; the activity is never forced
   through a generic surface twice in a row without someone noticing the
   friction.
5. Platform before framework, typography before chrome, calm before
   clever.
6. Pain reorders the roadmap; taste belongs to the owner; quality is not
   negotiable (standards.md).

## 7. The four walls (the boundaries that do not move)

Everything above is the wide interior yomihon is free to shape. These
four boundaries are the parts it may not adjust, route around, or
"improve": each stands at an irreversible edge, so widening one is a
ruling for the owner, never a session's call. They are stated here in
product terms; the detailed contracts they compress live in the
functional spec, the vault model, the threat model, and the vault
contract (`vault-schema.toml`), and the rulings that shaped them are in
the decision log.

**Wall 1 — the write face is a single field.** The only thing yomihon
writes into the vault is one frontmatter field, `status`. A transition is
legal only when the vault contract's state machine accepts it — the
move's `from` state and the acting owner both check out — and each
accepted transition becomes exactly one Git commit authored under Koopa's
own identity. Writing any other field, or widening what the write face
can touch, is not a feature but a constitutional amendment: shaped in §4,
ruled only by Koopa.

**Wall 2 — the process serves only the local machine.** The listener is
fixed to loopback (`127.0.0.1`); only the port is configurable, and
yomihon never serves or exposes the vault or any data derived from it
beyond the machine. Exactly three outbound exceptions are authorized, and
nothing else leaves without a new ruling. The three share only this:
each is triggered by an explicit action, each may reach only the fixed
Gemini provider API surfaces authorized for that exception, and each
validates its own approved input class at that final boundary — the
guarantees below belong to each exception, not a common filter run over
all three:

- **Document embeddings (D32).** When the vault contract's privacy policy
  permits it, eligible instance note content is sent to the provider's
  fixed `embedContent` endpoint to compute search vectors, which are then
  stored locally. Contract-declared private paths (`[privacy]` never-egress
  directories, D18) and non-instance artifacts (D47) are excluded from this
  payload, and the note's privacy allowance, its contract-source freshness,
  and the exact submitted bytes are revalidated immediately before the send.
  Semantic search is optional and bring-your-own-key — yomihon bundles no
  credential and runs no shared proxy.
- **Semantic query text (D50.1).** An explicit, applicable semantic search
  action sends its query text to that same fixed `embedContent` endpoint,
  at most once per action, and only after a current, compatible generation
  and its corpus are admitted. The raw query is owner-typed text, not vault
  content, so the D18/D47 path exclusions do not describe it; its
  protection is that it never enters yomihon's logs, caches, errors,
  metrics, or traces. Ordinary reading and lexical search send nothing.
- **Developer certification (D57).** An opt-in, test-only certification
  action reaches only two fixed Gemini endpoints, never a
  caller-configurable destination: its synthetic embedding inputs go to
  that same fixed `embedContent` endpoint, and its synthetic protocol probe
  may additionally call the fixed `countTokens` endpoint. It sends only
  repo-owned synthetic fixtures — the hard-coded protocol probes are fixed
  synthetic text, and the committed evaluation corpus paths and queries each
  carry the repository's synthetic marker. No argument or environment value
  can supply arbitrary text, a vault root, a vault path, or vault bytes, and
  it reads no live-vault D18/D47 authority because it never touches vault
  content.

**Wall 3 — the schema is understood from one source.** The single source
of schema truth is the vault contract, `vault-schema.toml`; one package
reads it, and no enum or state machine is ever copied a second time
anywhere. Semantic and AI capabilities may rank, summarize, and suggest,
but they never own schema truth, membership, privacy, mode existence,
content hiding, status, or a write.

**Wall 4 — the reader reports, it never repairs.** The renderer reads
fault-tolerantly and surfaces what is wrong — malformed YAML, broken
links, colliding names — as diagnostics; the judge reports and does not
edit. Correcting a note is a human act performed in the authoring tool,
never a silent fix by yomihon.
