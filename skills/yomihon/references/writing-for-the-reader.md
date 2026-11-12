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
   contract's answer type. If you want to answer one, write another note that
   cites it through the contract's source field. *Check:* the change you hand
   over touches no file of that type.
2. **Replace, never delete.** A note that supersedes another says so in the
   fields the contract declares for supersession, and the note it replaces is set
   to its archived status. *Check:* `yomihon check --deny warn --all <path>`
   exits 0.
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
   and its source goes in the contract's source field. It is for looking up, not
   for working through.

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
6. **A report that replaces another** names it in the supersession field, as for
   any note. A report outside the scanned directories is not checked by
   `yomihon check`, so this one is yours to keep.

## What not to write

- **A summary kept apart from the body** — a `summary` or `claim` field. The
  report's first paragraph is the summary; a copy drifts from it.
- **A number as a rule** — "at most five notes per map", "five minutes a
  section". They were once one person's habit and an agent will apply them as
  law.
- **The reader's thoughts.** A confusion is the reader's to say, in their own
  words, at the place they read it.
