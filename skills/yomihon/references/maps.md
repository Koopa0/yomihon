# Maps: grouping notes without putting them in order

**The question this answers:** what is a map in a yomihon vault, what does
writing one and listing a note on one actually change, and why does nothing
ever complain about a map?

A study path says what to read *next*. A map says what belongs *together*. The
two are different note types, declared separately, and a map reads none of the
study-path grammar.

The commands below are run against `examples/vault`, the small vault this
repository ships, because its numbers can then be quoted exactly. Every one of
them works the same against your own vault with `--root` pointed at it; only
the figures change.

## A map is declared, not recognised

A note becomes a map when its `type` is listed under the contract's
`[navigation] map_types`. This is the same silent gate that stands in front of
study paths: a note whose type is not listed is read, linked and searched like
any other note, and is not a map, however map-shaped its body is. Nothing
reports it, because nothing is wrong with it.

In this repository's `examples/vault` the contract says:

```toml
[navigation]
path_types = ["study-path"]
map_types = ["moc"]
```

so a map is a note carrying `type: moc`. See the tally for yourself:

```sh
yomihon serve --root examples/vault
```

The first lines it logs name the counts, and the maps are listed at
<http://127.0.0.1:9610/maps>:

```
level=INFO msg="vault snapshot built" files=36 ... paths=2 maps=2 ...
level=INFO msg="yomihon serving" addr=127.0.0.1:9610 vault=…
```

Delete the whole `[navigation]` table and the same command logs `paths=0
maps=0`: the table is all-or-nothing, and a fault in either key closes both.
[`frontmatter.md`](frontmatter.md) owns that table.

A map usually spans subjects, so a map type normally belongs on
`[fields] domain_exempt_types` — otherwise every map has to claim one domain.
The example contract lists `["study-path", "moc"]` there for that reason, and
you can watch the exemption do its work: `Maps/yomihon.md` carries no `domain`
and `check` is quiet about it, but drop `moc` from that list and the same note
reports `schema.required` saying `domain is required`.

## A map is not a study path

- **No order, no count, no prev/next.** A map has branches; it has no main
  line. Nothing in it is a lesson.
- **The `{sequence=…}` marker declares nothing here** — and does not show up
  either. [`study-paths.md`](study-paths.md) owns exactly which headings and
  rows it is stripped from, and the one place it survives.
- **Both are gated by the same table, and a type may not be on both lists.**
  Add `study-path` to `map_types` and the vault loses both: the same startup
  line then reads `paths=0 maps=0`.

## What becomes a branch, and what becomes an entry

A map's tree is read out of the body alone:

| | |
|---|---|
| A branch | a heading from H2 to H6. An H1 is treated as the note's title and opens nothing |
| An entry | a `[[link]]` under the open heading that resolves to exactly one governed note — in a list row, a checkbox row, prose, a table cell, or the heading itself |
| Never an entry | a link inside a code fence, a code span, an authored HTML block, or `%%…%%`; and a link written before the first heading, which no branch is open to hold |
| Dropped from the tree | a name that resolves to nothing, a name two files answer to, and a target inside a directory `[artifacts] non_instance_dirs` names |
| Dropped from the page | a heading with no entry of its own and no descendant carrying one — a heading of pure prose never appears |

Two of those differ from a study path and are worth holding on to. A checkbox
row (`- [ ] [[Note]]`) is a perfectly good map entry, while a study path never
counts one as a lesson. And a link sitting loose in a paragraph is an entry
here, while a study path only reads a list row that opens with its link.

Each line of that table is one small map away from being checked: write a map
whose only heading holds the shape you are unsure of, and read its count on the
index. A map with one heading and a link in a table cell reads **1 branch**;
the same map with the link in a code span, in a `<div>`, under `%%…%%`, inside
a fence, or pointing at a note under `System/templates` reads **0 branches**.

The count beside each map on the index — *5 branches*, *5 枝* — is the number
of branches left after that pruning, nested ones included. Run it against a map
written to exercise every case and the number is the test:

```sh
cp -R examples/vault /tmp/lab
cat > "/tmp/lab/Maps/Map counting.md" <<'MD'
---
title: Map counting
type: moc
status: ready
---

## Only an unresolved name

- [[Nothing answers to this]]

## Only prose

Words with no links.

## A parent with nothing of its own

### A child that carries one

- [[Two languages]]
MD
yomihon serve --root /tmp/lab
```

The index reads **2 branches** for that map: the unresolved name leaves its
heading empty and prunes it, the prose heading never had an entry, and the
parent survives only because its child carries one.

## `map_kind` buys exactly one thing

`[enums] map_kind` declares the legal values, and a note writing one outside
that list gets `schema.enum`, an error, the same as any other enum. Add
`map_kind: shelf` to the map above and check it:

```
{"rule_id":"schema.enum","severity":"error","path":"Maps/Map counting.md","field":"map_kind","message":"map_kind \"shelf\" is not an allowed value","source_rule":"vault-schema.toml","target":"shelf", ...}
```

That is the whole of it. Nothing else in yomihon reads the value — no page, no
count, no command behaves differently for `map_kind: topic` than for any other
declared word. It is a vocabulary your vault keeps for itself, and the contract
is what holds you to it.

## Nothing reports a map's shape

A map has no rule of its own. The two whose names open with *map* judge study
paths instead, which [`study-paths.md`](study-paths.md) explains. A map's links
are judged like any other note's — a dead `[[link]]` is an ordinary
`link.broken`, reported at its line — and its structure is judged not at all.
The map written above has a heading with no links, a heading whose only link
resolves to nothing, and a heading with no entry of its own, and

```sh
yomihon check --root /tmp/lab --format json "Maps/Map counting.md"
```

prints one line about none of them: the broken link. **So a map that projects
nothing is silent.** The way you find out is the index, where such a map reads
*0 branches*, and not a report.

## What listing a note on a map does change

`yomihon coverage` is the one command a map's contents move. It classifies
every `concept` note by what reaches it:

| State | Reached by |
|---|---|
| mounted | at least one note whose type is a declared map type |
| pending-mount | only notes that are not maps |
| orphan | nothing at all — the only real problem |

```sh
yomihon coverage --root examples/vault --format json
```

```
{"total_concepts":1,"domains":[{"domain":"yomihon","concepts":1,"mounted":1,"pending_mount":0,"orphan":0}],"pending_mount":[],"orphans":[],"unrouted":[]}
```

Add a second concept nothing links, and it appears under `orphans` with the
domain row reading `2 concepts: 1 mounted, 0 pending-mount, 1 orphan`. Move
`Maps/yomihon.md` to a directory `[scan] knowledge_dirs` does not name and the
mounted count drops to **0**: a map outside the knowledge layer cannot mount
anything, and neither can a note the contract withholds.

Two limits worth knowing before you read a number here. `concept` is yomihon's
own spelling and cannot be renamed: your contract decides whether the type
exists, not what it is called. A vault that declares no `concept` type is told
so rather than handed a tally of zero —

```
{"total_concepts":0,"domains":[], ... ,"not_applicable":"contract declares no \"concept\" type, so there is no concept corpus to judge"}
```

And `coverage` never gates: it exits 0 whatever it finds, and 2 only when it
could not run.

## Before calling a map done

1. The note's `type` is one the contract lists under `[navigation] map_types`.
2. The map appears at `/maps`, with the branch count you expected. A heading
   you meant to be a branch and cannot find there carries no resolved entry.
3. `yomihon check --root <vault> --format json <the map>` is quiet — which
   tells you about its links, and nothing about its shape.
4. If the vault keeps concepts, `yomihon coverage` shows no orphan you did not
   intend.
