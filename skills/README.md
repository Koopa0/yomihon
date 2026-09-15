# Skills

A skill is a folder of Markdown an agent reads before it starts work. The
folder's `SKILL.md` opens with a short frontmatter block — a name, and a
description that says both what the skill covers and the conditions that should
bring it to mind, because the description is the whole of what an agent weighs
when it decides whether to open the folder at all — and continues with a body
the agent follows as instruction rather than reads as an article. Installing
one adds no capability and changes no model. What it changes is what the agent
reaches for, and when: an agent carrying the skill below settles a note's
filename before its title, opens the vault's contract before choosing
frontmatter, and runs a command to see whether the note landed where it meant
it to, instead of assuming it did.

This folder holds one skill. It is written for the agent that writes notes
*into* a vault yomihon reads — not for anyone changing yomihon itself. The
maintainer's own agent configuration is not in the repository at all; it is
ignored, it answers a different question (how to change this project's Go), and
nothing here repeats it.

## What is in it

- [`yomihon/SKILL.md`](yomihon/SKILL.md) — the entry point: what yomihon
  projects out of Markdown, what has to be declared rather than inferred, the
  dialect the renderer treats specially, the command that settles any
  disagreement, and the shape of taking one note from prose into a course. It
  names every decision; it does not carry every subject.
- [`yomihon/references/`](yomihon/references/) — six files, one per subject,
  each the authority for what it covers: the frontmatter contract, how a name
  resolves, the study-path grammar, how a map works, the diagnostics, and the
  worked example at length. Each opens by saying which question it answers.

Every rule has exactly one file that states it in full, and the others point
there instead of restating it. That is not the same as saying nothing appears
twice — the entry point names a great deal in a sentence each, and has to. What
it should never do is explain the same rule a second way; where you find two
explanations of one rule, one of them is the bug.

Some of what the skill quotes lives outside this folder and cannot be checked
from inside it: `examples/vault` in the repository above, which every quoted
count and JSON line comes from, and the two tests below. A reader who has only
the skill can still act on every rule in it; they just cannot re-derive the
figures.

The skill describes yomihon's behaviour, which lives in the Go alongside this
folder and is pinned by that code's own tests — two of which read these files
directly. One holds the callout vocabulary `SKILL.md` lists to the one the
renderer answers to; the other holds every rule id named anywhere in this
folder to the set the checks emit, in both directions, so a rule cannot be
renamed without the mention going stale, and cannot be added without a reader
being left with nowhere to look it up. It does not describe any particular
vault: the types, the statuses, the fields and the directory layout are declared
per vault in a contract file, and the skill's steady refrain is that the
contract decides and the code never guesses.

## Installing it

You will need a `yomihon` on your `PATH` as well as the skill: its closing
checklist is mostly commands, and it has no `--version` to introduce itself
with. The skill's "Prove your instrument" section is the substitute, and a
narrow one — it asks a single rule whether this binary can see it, which tells
you the course rules are alive and nothing about the other forty.

To try the skill without placing anything, point Claude Code at the clone:

```sh
claude --plugin-dir ~/src/yomihon        # wherever the clone is
```

The repository root carries a plugin manifest beside this folder, so that loads
the skill for one session and leaves nothing behind to undo.

Two commands install it for good, with no clone to keep and no path to get
right:

```sh
claude plugin marketplace add Koopa0/yomihon
claude plugin install yomihon@yomihon
```

The repository is its own marketplace: a second manifest beside the plugin one
offers this single plugin, whose source is the repository root. `claude plugin
list` then names it and the version it installed, and `claude plugin uninstall
yomihon@yomihon` followed by `claude plugin marketplace remove yomihon` undoes
both steps. This route fetches the skill itself, so it is the one to take when
the machine that writes your notes holds no clone of this repository; the
symlinks below are for when it does.

To have an agent carry it every session, install it where the agent that writes
your notes runs — beside the vault, not beside yomihon's source. Run these from
that directory, with `YOMIHON` set to wherever you cloned this repository:

```sh
YOMIHON=~/src/yomihon        # wherever the clone is

# Claude Code, for one project
mkdir -p .claude/skills
ln -s "$YOMIHON/skills/yomihon" .claude/skills/yomihon

# the generic location other agents read
mkdir -p .agents/skills
ln -s "$YOMIHON/skills/yomihon" .agents/skills/yomihon
```

The symlink target must be absolute, or it will resolve against the directory
holding the link and dangle. Check **both** links, in one command, so a second
one that dangles cannot pass on the first one's success:

```sh
ls .claude/skills/yomihon/SKILL.md .agents/skills/yomihon/SKILL.md
```

Both paths have to print. `ls` exits non-zero and names the one that is missing
if either link is broken, which is the whole point of listing them together —
checking one and trusting two is how a dangling link survives an install.

A symlink keeps one copy, so a `git pull` that updates the skill updates what
the agent loads. Copy the directory instead when the agent runs somewhere the
clone is not, and re-copy when you update.

Install the whole directory either way. `SKILL.md` names its reference files by
relative path, so a `SKILL.md` on its own loses the depth it points at.

## Checking it took effect

Ask the agent something only the skill answers, in a session where you have not
pasted the answer:

- Which three values does a `{sequence=…}` marker take, and in which two places
  is it read?
- What does a `[[wikilink]]` resolve against, and what does it never resolve
  against?
- A lesson reads correctly in Obsidian but appears in no course. Name two
  things that could be true.

An agent that has loaded the skill answers the first with *primary, local and
none*, read on a heading from H2 to H6 and on a list row that has a child list
beneath it; and the second with the note's filename and path forms plus any
declared aliases, never the frontmatter title.

For the third, any two of these are right, and the third of them is the one the
skill calls the fault most worth fearing when you are adding a lesson:

- **the study path's own type is not on `[navigation] path_types`** — note that
  it is the *path's* type that has to be listed, never the lesson's, so "the
  lesson's type is not on the list" is a wrong answer that sounds like a right
  one;
- the branch the row sits on declares no `{sequence=…}`, so nothing is
  classified and nothing projects;
- the row does not open with its `[[link]]` — put anything visible in front of
  it and the row still reads perfectly while the course count silently drops.

An agent that answers vaguely, or reaches for general Obsidian knowledge, has
not loaded it.

The stronger check is the skill's own standard: given only this folder, an agent
should be able to say what the skill is, when it applies, what it changes about
how it writes, and walk one note from prose into a course. If it cannot do the
last one, it did not read
[`yomihon/references/worked-example.md`](yomihon/references/worked-example.md).
