---
title: Reading Fidelity
type: concept
domain: golang
---

The reading-fidelity fixture: the three things a note can say that a browser
has to receive intact — a footnote, a link written at a section, and code.

A conclusion bounded by the study's scope[^scope], and cited a second
time[^scope] further along the sentence.

A footnote nobody defined[^undefined] stays exactly as it was written.

See [[Glass Tide#第三節：失約的燈]] for the lamp,
[[Glass Tide#Sensory material|back to the material]] for the rest,
[[Glass Tide#Fourth-level landing]] for a heading written at ####, and
[[Glass Tide#Glass Tide]] for the note as a whole — that last one names the
heading the destination removed as a duplicate of its own title.

A fourth is written at [[Glass Tide#A section nobody wrote]], a name that note
does not answer to, so it arrives marked as a link that will land somewhere
other than where it says.

A fifth points out of the vault altogether, at <https://example.invalid/lamps>,
which is a page this machine can neither read nor preview.

A sixth names a vault file that is not a note at all, [[plain.txt]], which has a
page of its own and nothing a preview could cut a section out of.

A seventh returns to a marked line in that same note,
[[Glass Tide#^tide-wrap-1|back to the marked line]], so a block address can be
followed the way a section can.

An eighth returns to a marked list item,
[[Glass Tide#^tide-item-1|back to the marked item]], so a block address that is
not a paragraph is followed the same way.

Quoting a note that has footnotes of its own puts a second set on this page,
numbered separately because those definitions live in the note they came from.

![[beta]]

> [!note] A callout with a note of its own
> The callout is read as part of the note it is written in, so its
> footnote[^aside] joins the note's own numbering and its one endnote list.
>
> [^aside]: The callout's own footnote.

```go
// countLamps reports how many lamps a listing says are lit.
package tide

import "net/url"

func countLamps(listing string) int {
	parsed, err := url.Parse("https://example.invalid/lamps?lit=1")
	if err != nil {
		return 0
	}
	return len(parsed.Query()) + len(listing)
}
```

```diff
--- a/tide.go
+++ b/tide.go
@@ -1,3 +1,3 @@
-	return 0
+	return len(listing)
```

A task list is two markers if the sheet forgets: a disc and a checkbox.

- [ ] Unfinished task
- [x] Finished task

An ordinary list still owes its disc.

- an ordinary item

[^scope]: Only the scope of this study is covered.
