# Names, and why a `[[link]]` misses

**The question this answers:** what does yomihon key a note by, what does a
`[[wikilink]]` resolve against, and what do I do about the four ways it fails?

This is the part of the dialect where generic Obsidian knowledge will let you
down, so it is worth reading rather than assuming.

## See the keys for yourself

The entry point states the rule. Here is the experiment that shows it, which
you can run in a minute against `examples/vault`. Put these eight links in one
note and run `check` over it:

```markdown
1. [[The vault contract]]
2. [[The vault contract.md]]
3. [[Notes/The vault contract]]
4. [[Notes/The vault contract.md]]
5. [[tHe VaUlT cOnTrAcT]]
6. [[The Vault Contract]]
7. [[  The vault contract  ]]
8. [[contract]]
9. [[The contract]]
```

Eight resolve. Only the ninth is reported, because it is a name no file answers
to. Lines 1–4 are the four forms of the note's location; 5–7 are the same key
after the fold — trimmed, NFC, compared without regard to case; 8 is one of the
two aliases that note declares in its frontmatter.

Two consequences are easy to miss. Because the fold happens before the
comparison, two files whose names differ only in case collide with each other
on any filesystem, not only on a case-insensitive one. And every note in this
example vault happens to be titled exactly what it is filed under, which is why
nothing here shows the title trap — write a note whose `title` and filename
differ, link it by the title, and you get `link.title_not_alias`, a finding
that names the note you meant and offers the two repairs.

## The three answers

A name resolves to exactly one file, to several, or to none. yomihon never
picks between several:

| Outcome | What the page shows | What `check` says |
|---|---|---|
| one file | an ordinary link | nothing |
| several | nothing is linked; the name is marked ambiguous and the candidates are listed in place | `collision.name`, once for the whole collision, with every path in `collision_members` |
| none | the link is marked where it sits, with the reason | `link.broken` — or `link.title_not_alias` when the name is some note's title |

Two notes declaring the same alias is `collision.alias`, and has the same
effect: the alias resolves to nothing at all, because an alias that two notes
answer to names neither.

The repairs, in the order to try them: give the note a name unique in the
vault; add an `aliases` entry when a second spelling genuinely should work;
link by full vault-relative path when a generic name is unavoidable.

## Fragments

A target may carry a display label, a section, or a block address. They are
split in a fixed order — `|` first, then `#`, then `^` — so a label containing
a `#` behaves the way you would want and not the other way round.

| Written | What happens when the fragment misses |
|---|---|
| `[[Note\|shown words]]` | — |
| `[[Note#Heading]]` | the note still links **and the address is kept**: the rendered link is `…/Note.md#heading`, pointing at an anchor that is not there. `link.section_missing`, with `resolved_to` naming the note |
| `[[Note#^block-id]]` | the fragment is **withdrawn**: the rendered link is `…/Note.md` with nothing after it, and leads to the whole note. `link.block_missing` |

That asymmetry is deliberate and is worth remembering, because the two failures
look identical in the source and behave differently in the browser.

A note's own headings become anchors with CJK intact; a repeated heading slug
gets `-2`, `-3` appended until it is free. To make a line addressable, end it
with `^my-id`.

## Embeds

`![[Note]]` pulls the note's body in, and `![[Note#Heading]]` only that
section. Embedding is one level deep: an embed inside an embedded note is not
expanded further. An embed whose section or block is not found shows nothing of
the note — a notice names the address that failed and links the note —
reporting `embed.section_missing` or `embed.block_missing`.

## Links that are not wikilinks

A plain Markdown link to a path inside the vault is checked too: a path that is
not there is `link.broken.path`. A remote destination is never fetched.

## Naming a link as owed rather than broken

A vault is written forward. You link a concept from where it belongs long
before the file exists, and that link is tracked, not broken. Two ways to say
so, and both drop the finding from `warn` to `info`:

**Under a heading containing a gap word.** The softening runs from that heading
down to the next heading at the same or a higher level, and the plain-text
names listed under it become planned names for the whole vault. The finding
changes its `evidence` as well as its severity, to *a tracked forward-reference
(under a gap heading or listed as a planned concept)*.

**Beside a gap word on an ordinary line.** Every `[[name]]` written on that same
line is marked planned across the vault. A bare name with no brackets is not
collected.

Three things about this that a reader gets wrong:

- **The words are not built in.** They are `[rules] planned_gap_marks` and
  `[rules] planned_inline_marks` in your contract. A contract that omits them is
  loaded with defaults — `缺口 待補 待寫 待整理 待建` for headings, `待整理 待建
  下一課` for lines — which is why they look like product behaviour. A contract
  that overrides them softens on entirely different words, and one that writes
  an empty list softens nothing. Check yours before relying on a word.
- **The match is a substring, with no word boundary.** A heading that merely
  happens to contain one of those words turns its whole section into a planned
  gap, and every broken link beneath it silently drops to `info` — which
  `--deny warn` does not catch. This is the most expensive accident in the
  dialect: the gate stays green and the links stay dead.
- **It does not soften what a course promises.** A lesson a study path lists and
  the vault does not have is still reported. A course is answered where the
  course says it is.

## If a link is missing and you cannot see why

Work down this order, because the cheap causes are the common ones: is the
target spelled the way the *file* is spelled, not the way the note is titled?
Does any other file in the vault claim the same name under the fold? Is the
thing after `#` a heading that really exists? And — the one people reach for
last — is the link under a heading whose wording has quietly made it a planned
gap, so the finding you are looking for was downgraded to `info` and your
`--deny warn` never saw it?
