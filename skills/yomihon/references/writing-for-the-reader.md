# Writing for the reader

This file answers one question: how should a note be written so that the person
who reads it can learn from it, or check it?

The rest of this skill is about being understood by yomihon. This file is about
being understood by the person who reads what you wrote. They are usually the
person who sent you to write it, they read it later and without you, and they
decide from it what to trust, what to learn and what to ask next.

Two kinds of note are read two ways, so they are written two ways. Decide which
one you are writing before you write it.

- **A note the reader learns from** — a lesson, or a note a lesson's reader is
  meant to work through. They should reach the answer, not be handed it.
- **A note the reader consults or checks** — a report, or a concept they come
  back to look up. They should find the answer first, and be able to check it.

The two kinds' headings are opposites on purpose. A lesson's heading is the
situation or the question; a report's heading is its conclusion. Do not carry
one kind's habit into the other.

`yomihon check` does not test any rule on this page. They are for you to meet
and for a second reader — a person, or a second agent that did not write the
note — to verify by reading. Where a rule ends in *Check:* a one-line test
exists.

## For every note

1. **Leave the reader's own writing alone.** Do not create or edit a note of the
   type that holds the contract's `thought` role. If you want to answer one,
   write another note that cites it in `based_on`. *Check:* the change you hand
   over touches no file of that type.
2. **Replace by deleting, and leave the old name behind.** A note that another
   note replaces is deleted, and its old file name goes into the successor's
   `aliases`, so a `[[link]]` written to the old name still arrives. That is
   only safe under four conditions, and when one fails you do not delete:
   - *The successor says the same thing.* An alias says "this is that". A
     successor that is wider, narrower, a merge or a split is not that: add no
     alias, and rewrite each note that cited the old one, one by one.
   - *No thought cites the note.* A reader's thought is evidence about the
     words it cites, and an alias would quietly point it at other words. Judge
     this after aliases are resolved, the way yomihon resolves a link: if a
     thought's `based_on` names the note, by its file name or by an alias the
     note already has, the note stays, with or without a successor.
     `provenance.alias_resolved` reports a thought an alias has already
     redirected.
   - *The vault is under version control.* Deleting is how the old text is given
     up, and git is the only place it stays.
   - *Every inbound reference is listed first.* List each wikilink,
     `[[Note#Section]]`, `[[Note#^block]]`, embed and relative Markdown link
     that reaches the note, and mark each *keep*, *rewrite*, or *rewrite,
     meaning changes*. `check` stops the structural breaks —
     `link.broken.path` for a relative link, which an alias does not rescue,
     and `link.section_missing`, `link.block_missing`,
     `embed.section_missing` and `embed.block_missing` for a fragment the
     successor lacks — but nothing catches a link that still resolves to words
     that now say something else. Whether the meaning changes is an editorial
     decision: do not rewrite those mechanically, and give the list to a person
     to confirm.

   *Check:* the list exists, `yomihon check --deny warn --all <the notes that
   cited the old one>` exits 0 after the deletion, and no thought is reported by
   `provenance.alias_resolved`.
3. **Mark inference.** A judgement that rests on no source in the note starts
   with "Inference:" or says what it rests on.
4. **Do not grade the reader.** No "mastered", "complete", "well done", and no
   count of what they have read. *Check:* search the note for those words.
5. **Give each link its reason.** The sentence a `[[link]]` sits in says why to
   follow it; a bare "Related:" list is not a reason.
6. **Write dates as dates.** `2026-11-12`, never "last week" or "recently".

## For a note the reader learns from

The rule in *What a lesson asks its reader* stands: the prompt is open prose
outside the fold, the answer inside it. The rules below say what else keeps the
reader from being handed the answer.

1. **A lesson's heading is a situation, a task or a question** — never the
   conclusion, and never a word that appears only inside a fold's answer.
2. **No summary on top.** Do not open with key points or a conclusion. If a
   summary helps, put it at the end, folded. *Check:* no heading or callout near
   the top is titled summary, key points or the like.
3. **Run the code, and show what it printed.** Every code block that can be run
   is followed by the command and its actual output.
4. **State a limit.** Each lesson has at least one sentence, written as a
   negation or a counter-example, that says when what it taught does not hold.
5. **Language lessons.** Text in the language being taught carries `lang`;
   contrasts the learner will confuse (に／で, は／が) sit in one note, side by
   side; a heading says what the learner will be able to do ("ask where
   something is"), not the grammar's name.
6. **A concept note defines one concept**, its first sentence is the definition,
   and its source goes in `based_on`. It is for looking up, not for working
   through.

## Teaching principles

These hold for every lesson you write and for how you read what a reader left
open. They are judgements, so nothing checks them.

1. **Do not cut a coherent explanation into pieces to create recall prompts.**
   A connected explanation stays connected; a question is there to make the
   reader decide something, not to make them fetch what the paragraph above
   just said.
2. **When the prerequisites are missing, explain or draw; do not ask.** A
   question about something the reader has not met tests nothing and only
   costs them. Explain it, or draw it, and let the question come once they have
   the pieces.
3. **A reader's confusion is a lead to investigate, not a verdict on the
   lesson.** A location marked "Not sure yet", or a thought that stays a draft,
   tells you where to look. Read the passage, and the reader's words where they
   wrote any, before deciding whether the gap is in the lesson, in a
   prerequisite, or in a question the reader has not asked yet.
4. **Reading that leaves no trace is complete reading.** No mark and no thought
   is evidence of nothing, so an empty `left-open` does not say the reader
   understood or that they did not read, and no lesson asks the reader to leave
   traces.

## For a report

1. **The first paragraph is the answer:** at most three sentences, and the
   conclusion is in them, not the background.
2. **Each heading is a complete sentence with a verb.** A heading that is a noun
   phrase labels; it does not say anything.
3. **Attach evidence to the claim.** An external fact — a version, a number, an
   API's behaviour — carries a link or a footnote in the same section. The last
   section, "Evidence", lists them as links. *Check:* the section exists and
   every entry in it is a link.
4. **Say what is not established.** One section, "Assumptions and limits", with
   at least one entry; if there is none, it says "None."
5. **When two sources disagree,** say what each was answering, and only then
   compare them.
6. **A later report cites the earlier one instead of replacing it.** Reports
   are dated, so the earlier one stays; the later one links it with `[[…]]` and
   says in a sentence what it changes.

### Where a report goes

A report is a Markdown file written directly in `System/reports/`. That one
folder is the only place yomihon looks: a file in a folder below it is not a
report, and neither is a file of another kind.

- **Dated, newest first.** The reports page lists them by day, newest first. The
  day is the report's `created` field, which the contract has to list in
  `[fields] common`, and failing that a date the file name begins with, such as
  `2026-11-12 Postgres checkpoints.md`. A report with neither is listed after
  the dated ones, marked "No date". Write `created` in every report.
- **Called by its title.** The title is the first `#` heading, or the file name
  without its extension. Two reports sharing a title are told apart by their
  day, or by their file name when the day is shared too.
- **Organised by the links it carries.** There is no index and no sorting into
  folders. A report links, with `[[…]]`, the lessons and concepts it is about,
  and a roll-up is one more report that cites the earlier ones; the notes that
  cite a report are listed beside it.

## What not to write

- **A summary kept apart from the body** — a `summary` or `claim` field. The
  report's first paragraph is the summary; a copy drifts from it.
- **A number as a rule** — "at most five notes per map", "five minutes a
  section". They were once one person's habit and an agent will apply them as
  law.
- **The reader's thoughts.** A confusion is the reader's to say, in their own
  words, at the place they read it.
