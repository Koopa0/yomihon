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
Obsidian renders *files*. yomihon renders *the work*: what changed, what
needs a decision, what an agent claims, how knowledge connects, how far a
course of study has come. The seal — reading a thing and ruling on it in
the same breath — is the product's heart, and everything else is built
around getting the owner to that moment with less friction and more
context.

It is deliberately **not an editor**. Authoring and revising prose belongs
to Obsidian, which is excellent at it; yomihon writes exactly one
frontmatter field under a state machine (wall 1) and renders everything
else faithfully (wall 4). This division is not a limitation to apologize
for — it is the product architecture: the editor and the terminal stay
different tools with different postures, and each is better for it.

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
| **Study** | Working through lessons, drilling patterns, sealing what is finished | Reading page + five interactions; syllabus wayfinding (Here, auto-opened paths); the seal | Smoothness batch (§12), hover layer (§11), view transitions |
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
   editor. Until the ruling, capture stays in Obsidian and this entry
   exists so the idea has a disciplined landing zone instead of an
   improvised one.

Reading the journal in yomihon needs no amendment — local rendering is not
egress (D39/D42 guard the machine-readable outputs, not the owner's own
eyes).

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
- **Chrome in English, content in its own language** (D28): the interface
  whispers, the vault speaks.
- **The design bundle remains the visual source** for components; this
  charter governs what new surfaces may look like when the bundle is
  silent.

## 6. Product principles (the short list)

1. The seal moment is sacred: fewer steps to it, more context around it.
2. Terminal, not editor — capture may someday enter by amendment; editing
   never.
3. Every capability gets its human surface and its agent surface (roadmap
   §6); neither is an afterthought.
4. The interface adapts to the activity; the activity is never forced
   through a generic surface twice in a row without someone noticing the
   friction.
5. Platform before framework, typography before chrome, calm before
   clever.
6. Pain reorders the roadmap; taste belongs to the owner; quality is not
   negotiable (standards.md).
