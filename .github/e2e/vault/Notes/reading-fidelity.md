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

See [[Glass Tide#第三節：失約的燈]] for the lamp, and
[[Glass Tide#Sensory material|back to the material]] for the rest.

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

[^scope]: Only the scope of this study is covered.
