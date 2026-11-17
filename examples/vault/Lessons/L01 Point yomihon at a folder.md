---
aliases: [Building yomihon from source]
type: lesson
slug: l01-point-at-a-folder
level: fundamental
created: 2026-09-22
lang: en
---

Start the reader with a Markdown folder:

```sh
yomihon /path/to/notes
```

Open `http://127.0.0.1:9610` to read, browse folders and search. Starting the
server leaves the notes unchanged. Keep the command running while you read;
press `Ctrl+C` in the terminal to stop it.

[[The vault contract]] checks notes against declared types and rules, and
decides what the command line withholds. [[L02 Add a contract]] shows the setup.

| What works with no rules file | What a rules file adds |
| --- | --- |
| Reading, rendering, folders | The note types, fields and values `check` accepts |
| Wikilinks and backlinks | Directories the command line withholds |
| Full-text search | Your own names for courses, units, indexes and thoughts |
| Courses, units and indexes, by the type a note names | |
| Your marks and your thoughts | |
