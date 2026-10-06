# Make a course

English | [繁體中文](authoring.zh-TW.md)

You need a text editor and the [yomihon binary](https://github.com/koopa0/yomihon/releases). Reading and search work on any Markdown folder. A course also needs a contract that declares which notes are lessons and study paths.

## Get the starter

Open the release page for the binary you installed and download its **Source code** archive. Extract `examples/vault/System/schemas/vault-schema.toml` and copy it to `System/schemas/vault-schema.toml` inside your notes folder. You need neither Go nor a Git clone. For example, [the v0.2.0 starter](https://raw.githubusercontent.com/koopa0/yomihon/v0.2.0/examples/vault/System/schemas/vault-schema.toml) belongs to v0.2.0; use your own release's tag when it differs. When building from source, use [the starter in that checkout](../examples/vault/System/schemas/vault-schema.toml).

The starter is a decision to adapt, not a rule for every vault. Review its note types, fields, lifecycle transitions, scanned directories and privacy directories against your notes. In particular, `privacy.never_egress_dirs` withholds those directories from command output; it does not hide them from the local reader. yomihon reads this file and never writes it.

For this small example, keep the starter unchanged. Create `Concepts/`, `Inbox/`, `Lessons/`, `Maps/` and `Notes/` inside your notes folder, matching the starter’s scanned directories. The first, second and fourth can remain empty. Save the following four files in `Notes` and `Lessons`. The English filenames below are also the wikilink targets; a frontmatter title is not a substitute for a filename.

## Arrange the course

### `Notes/First course.md`

```markdown
---
title: First course
type: study-path
status: ready
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
title: Start
type: lesson
status: ready
domain: yomihon
slug: first-course-start
level: fundamental
lang: en
---

Read this first. What do you already know about the subject?
```

### `Lessons/Continue.md`

```markdown
---
title: Continue
type: lesson
status: ready
domain: yomihon
slug: first-course-continue
level: fundamental
lang: en
---

Try one small example using what you read in Start.
```

### `Lessons/Extra.md`

```markdown
---
title: Extra
type: lesson
status: ready
domain: yomihon
slug: first-course-extra
level: fundamental
lang: en
---

An optional explanation for readers who want more detail.
```

These values belong to the starter. For your own course, choose values your adapted contract allows. Keep each lesson's slug unique and stable, and set `lang` to its authored language.

## Read and check

Run `yomihon ~/notes`, substituting your folder. Open <http://127.0.0.1:9610/paths>, then **First course**. It has two main-line lessons and one optional lesson. Start's next lesson is Continue; Extra stays in its side branch.

Run `yomihon check --root ~/notes` to find broken links, frontmatter problems and undeclared course branches. Findings do not change your files; correct them in your editor. Without a contract the command refuses with exit 2, but `yomihon ~/notes` still reads the folder and <http://127.0.0.1:9610/health> shows link findings.

For more authoring shapes, see [the study-path reference](../skills/yomihon/references/study-paths.md).
