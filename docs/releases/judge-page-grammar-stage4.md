# Judge page grammar: Stage 4

`yomihon check` recognizes these references with the page's Markdown grammar.
Four previously uncovered shapes change the report:

- A wikilink in a fence's info line, such as the opener below, no longer
  produces `link.broken`. The fence quotes it.

  ````markdown
  ``` [[Missing]]
  quoted
  ```
  ````

- A missing wikilink in a referenced footnote's indented second paragraph now
  produces `link.broken` at its original line:

  ```markdown
  ref[^n].

  [^n]: first paragraph.

      [[Missing]]
  ```

- `- [ ](Missing.md)` consumes a task checkbox marker. It no longer produces
  `link.broken.path` for `Missing.md`.
- ``https://example.invalid/`Notes/Missing.md` `` is one autolink. Its backticks
  no longer produce an independent `link.broken.path` for `Notes/Missing.md`.

With `--deny warn`, the three removed warnings may change exit 1 to exit 0;
the new footnote warning may change exit 0 to exit 1, assuming no other denied
findings. Other warnings still gate normally. A successful report without
`--deny` exits 0 even when it contains findings. Tool errors remain failures
(CLI exit 2).

Unreferenced footnote definitions, such as `[^unused]: [[A]]`, contribute no
live citation to `check`. The page's diagnostic-only P0 disagreement for that
shape remains separately tracked under #1011; this stage does not claim full
page/check agreement.

Existing frozen fixture and golden bytes stay unchanged. Four new fixture
vaults and four new JSONL goldens document these categories, with an ordinary
missing-reference control in each vault. An ordinary Markdown file link still
warns, and a standalone slash-bearing backtick path still warns in `check`.
