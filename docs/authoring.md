# Make a course

English | [繁體中文](authoring.zh-TW.md)

You need a text editor and the [yomihon binary](https://github.com/koopa0/yomihon/releases). Reading and search work on any Markdown folder. A course also needs a contract that declares which notes are lessons and study paths.

## Get the starter

Open the release page for the binary you installed and download its **Source code** archive. Extract `examples/vault/System/schemas/vault-schema.toml` and copy it to `System/schemas/vault-schema.toml` inside your notes folder. You need neither Go nor a Git clone. Take the starter from the same release as the binary: a contract and a binary from different releases may not read each other, and the first line of the contract, `schema_version`, is what says so. The raw file for a release is at `https://raw.githubusercontent.com/koopa0/yomihon/<tag>/examples/vault/System/schemas/vault-schema.toml`, with that release's tag in place of `<tag>`. When building from source, use [the starter in that checkout](../examples/vault/System/schemas/vault-schema.toml).

The starter is a decision to adapt, not a rule for every vault. Review its folders, note types, field lists and privacy directories against your notes. In particular, `privacy.never_egress_dirs` withholds those directories from command output; it does not hide them from the local reader. yomihon reads this file and never writes it.

For this small example, keep the starter unchanged. Create `Concepts/`, `Inbox/`, `Lessons/`, `Maps/` and `Notes/` inside your notes folder, matching the directories the starter’s `folders.checked` names. The first, second and fourth can remain empty. Save the following four files in `Notes` and `Lessons`. The English filenames below are also the wikilink targets and the titles: a note's title is its first `#` heading, or its filename when it has none, and the frontmatter carries no title or domain.

## Arrange the course

### `Notes/First course.md`

```markdown
---
type: study-path
lang: en
---

## Main line {sequence=primary}
- [[Start]]
    - More detail {sequence=local}
        - [[Extra]]
- [[Continue]]

## Reference {sequence=none}
- [[Start]] — revisit when needed
```

The shortest main line is three lines: one heading ending in `{sequence=primary}` and two lesson rows. The side branch adds a separate row **under** `Start`; its `{sequence=local}` marker belongs on that row, not on `Start` itself.

| Marker | What the reader gets |
| --- | --- |
| `primary` | The main line, in written order; its lessons count toward the course total. |
| `local` | An optional branch with its own count and previous/next order; it does not join the main line. |
| `none` | Reference material outside the course count and previous/next order. |

Put a marker at the end of an H2–H6 heading or a row that opens a child list. Every branch declares its own role. A lesson row starts with one wikilink; commentary can follow it.

## Write the lessons

### `Lessons/Start.md`

```markdown
---
type: lesson
slug: first-course-start
level: fundamental
lang: en
---

Read this first. What do you already know about the subject?
```

### `Lessons/Continue.md`

```markdown
---
type: lesson
slug: first-course-continue
level: fundamental
lang: en
---

Try one small example using what you read in Start.
```

### `Lessons/Extra.md`

```markdown
---
type: lesson
slug: first-course-extra
level: fundamental
lang: en
---

An optional explanation for readers who want more detail.
```

These values belong to the starter. For your own course, choose values your adapted contract allows. Keep each lesson's slug unique and stable, and set `lang` to its authored language.

## Know the fields

The fields a note may carry are a closed list that yomihon defines: `type`, `aliases`, `draft`, `based_on`, `lang`, `created`, `slug`, `level`, `prerequisites`, `verified_on` and `verified_with`. Obsidian's own `tags` and `cssclasses` also pass. Your contract chooses which of them each type of note may write, and `yomihon check` reports any other key as `schema.unknown_key`. [The frontmatter reference](../skills/yomihon/references/frontmatter.md) says what each field does.

A note writes no title and no domain. Its title is its first `#` heading, or its file name when it has none. Its domain is the folder it sits in, below a root the contract names in `folders.domain_from`.

A note you have not finished says `draft: true`. Delete the line when it is done; a note with no `draft` is finished. yomihon marks drafts quietly and never edits your notes, so this change is yours to make in your editor.

## Replace a note

When one note replaces another, delete the old one and add its file name to the new note's `aliases`, so links written to the old name still arrive. Do it only when the new note says the same thing, your notes are in a Git repository, no thought of yours cites the old note in `based_on`, and you have looked at every note that links to it. `yomihon check` reports a link that breaks; it does not report a link that still resolves to words that now mean something else. [The writing rules](../skills/yomihon/references/writing-for-the-reader.md) give the whole rule.

## Read and check

Run `yomihon ~/notes`, substituting your folder. Open <http://127.0.0.1:9610/paths>, then **First course**. It has two main-line lessons and one optional lesson. Start's next lesson is Continue; Extra stays in its side branch.

Run `yomihon check --root ~/notes` to find broken links, frontmatter problems and undeclared course branches. Findings do not change your files; correct them in your editor. Without a contract the command refuses with exit 2, but `yomihon ~/notes` still reads the folder and <http://127.0.0.1:9610/health> shows link findings.

Quote references inside YAML lists: write `based_on: ["[[Note]]"]` or
`based_on: ["Note"]`. An unquoted `based_on: [[Note]]` is a nested YAML list,
so `yomihon check` reports `schema.reference_nested_sequence` once for that
field, and the note's page says the same in your language. Like the other
frontmatter rules, it judges only notes inside `folders.checked`. Edit the
reference in your editor; yomihon leaves its bytes unchanged.

For more authoring shapes, see [the study-path reference](../skills/yomihon/references/study-paths.md).
