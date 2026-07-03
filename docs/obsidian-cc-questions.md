# Items for the obsidian Claude Code session

> Purpose: Koopa brings this into a session in the vault repo and pastes it to the obsidian CC. Please write replies to a file (the vault side picks where); Koopa will bring them back.
>
> Requester context (two lines): kurodo (`~/go/src/github.com/koopa0/kurodo`) is the vault's local reading + adjudication interface, and the successor to yomihon and kura — until kura's retirement gate (M4) is reached, not one line of kura changes. kurodo's single source of schema understanding is `System/schemas/vault-schema.toml`; it is never hardcoded.

## 1. CLI requirements list (most valuable for the M4 interface design)

kurodo's M4 will absorb kura's `check` / `exists` / `coverage`, keeping the JSONL contract, `--format`, exit codes, and scan boundary byte-compatible. We want to know the **real-world usage** on the vault side:

- Which commands and flags do you (and the manual scenarios outside the hermes pipeline) actually use? How often? (`check --deny`, `exists` as a dedup oracle, `coverage`, `--format md`, `--all`, …)
- Are there gaps where "you always have to work around it"? Candidates: `backlinks <note>` (backlinks / blast radius), an orphans query, a frontmatter query (e.g., "list every lesson with status=draft"), and others.
- This only gathers requirements and prioritizes them; it is not a commitment list. Any expansion is scheduled through kurodo's yard.

## 2. The entry semantics of Vault-Index

kurodo's homepage (M2) wants to reuse the sections of `System/Vault-Index.md` directly as its information architecture (the four boards, the gap ledger, the domain MOC entries) rather than inventing its own homepage. Please point out:

- Which sections are **stable entry points** (worth turning into kurodo homepage blocks) and which are temporary?
- The role of the five `.base` files in `Views/` — for v0, kurodo does only "link back to open in Obsidian" for Bases; is that division of labor right?

## 3. The diary type + the privacy egress line (pending on the vault side — please draft)

kurodo is local-only (loopback is hardcoded) and will index **the entire vault** — so "which content never egresses and never enters agent context" needs a dedicated document before hermes's cloud lane starts up. Please have the vault side draft:

- The schema decision for `type: diary`: which folder it lands in, its status set, and whether it enters `[scan]` scope.
- The privacy-line document (suggested location: `System/agent-guides/`): which types/folders may be sent to the cloud brain and which never; whether kura/kurodo need a single machine-decidable rule.

## 4. Consolidate the slug rules into one page (direction is set — please write it up)

The four converged points Koopa has already nodded to: **only lessons need a slug; the format = namespace prefix + number (`jp-minna-lNN` / `jp-kana-pNN`, with Go lessons using a plain slug); once finalized it never changes; no other note needs a slug at all.**

Please write this into a one-page document, and confirm whether `vault-schema.toml` needs to be synced (right now the toml has only `slug_pattern`; the namespace-prefix convention still lives at the document level).

## 5. Hard rule: agents never call kurodo's write endpoint (please add to agent-guides and hermes's guidance docs)

kurodo's status flip auto-commits under Koopa's git identity. Within the local trust boundary, any same-account process (including an agent) that calls `POST /status` is recorded as a Koopa-authored adjudication — technically indistinguishable (curl carries no Sec-Fetch-Site header, and same-origin protection only blocks cross-site browser requests). So fix it with governance: please add a one-line hard rule — **"agents never call kurodo's write endpoint (`POST /status`); an agent's status proposals always go through file changes + the review pipeline."** This is the mechanical face of the same rule as the existing "any agent that writes `ready` is a violation" (the vault-schema.toml lifecycle owner).
