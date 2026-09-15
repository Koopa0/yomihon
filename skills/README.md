# Skills

A skill is a folder of Markdown an agent reads before it starts work. The
folder's `SKILL.md` opens with a short frontmatter block — a name, a
description of what the skill covers, and the conditions that should bring it
to mind — and continues with a body the agent follows as instruction rather
than reads as an article. Installing one adds no capability and changes no
model. What it changes is what the agent reaches for, and when: an agent
carrying the skill below settles a note's filename before its title, opens the
vault's contract before choosing frontmatter, and runs a command to see whether
the note landed where it meant it to, instead of assuming it did.

This folder holds one skill. It is written for the agent that writes notes
*into* a vault yomihon reads — not for anyone changing yomihon itself. The
maintainer's own agent configuration is not in the repository at all; it is
ignored, it answers a different question (how to change this project's Go), and
nothing here repeats it.

## What is in it

- [`yomihon/SKILL.md`](yomihon/SKILL.md) — the entry point, and enough on its
  own to write a correct note: what yomihon projects out of Markdown, what has
  to be declared rather than inferred, the dialect the renderer treats
  specially, the command that settles any disagreement, and one worked example
  end to end.
- [`yomihon/references/`](yomihon/references/) — five files the entry point
  sends you to when you need the whole of one thing: the frontmatter contract,
  how a name resolves, the study-path grammar, the diagnostics, and the worked
  example at length. Each opens by saying which question it answers.

The skill describes yomihon's behaviour, which lives in the Go alongside this
folder and is pinned by that code's own tests — two of which read `SKILL.md`
directly, so the rule names and the callout vocabulary it lists cannot drift
from the product without a test going red. It does not describe any particular
vault: the types, the statuses, the fields and the directory layout are declared
per vault in a contract file, and the skill's steady refrain is that the
contract decides and the code never guesses.

## Installing it

To try it without placing anything, point Claude Code at the clone:

```sh
claude --plugin-dir ~/src/yomihon        # wherever the clone is
```

The repository root carries a plugin manifest beside this folder, so that loads
the skill for one session and leaves nothing behind to undo.

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
holding the link and dangle. Check before you trust it:

```sh
ls .claude/skills/yomihon/SKILL.md
```

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
beneath it; the second with the note's filename and path forms plus any
declared aliases, never the frontmatter title; and the third with at least the
note's type not being listed under the contract's path types, and the branch
carrying no sequence declaration. An agent that answers vaguely, or reaches for
general Obsidian knowledge, has not loaded it.

The stronger check is the skill's own standard: given only this folder, an agent
should be able to say what the skill is, when it applies, what it changes about
how it writes, and walk one note from prose into a course. If it cannot do the
last one, it did not read
[`yomihon/references/worked-example.md`](yomihon/references/worked-example.md).
