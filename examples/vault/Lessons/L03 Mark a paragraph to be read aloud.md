---
title: L03 Mark a paragraph to be read aloud
type: lesson
status: ready
domain: japanese
slug: l03-read-aloud
level: intermediate
created: 2026-09-22
updated: 2026-09-22
lang: en
---

On a governed lesson, put this marker immediately above a Japanese paragraph:

```
<!-- read-aloud: ja -->
```

Example:

<!-- read-aloud: ja -->
<ruby>古池<rt>ふるいけ</rt></ruby>や<ruby>蛙<rt>かわず</rt></ruby><ruby>飛<rt>と</rt></ruby>びこむ<ruby>水<rt>みず</rt></ruby>の<ruby>音<rt>おと</rt></ruby>

Ruby readings are omitted from speech so pronunciation is not repeated.
The marker supports `ja` only.

> [!tip]- Where the marker does nothing
> The note must be a governed `lesson`, outside the template directories.
> Elsewhere the marker remains an ordinary comment.

[[芭蕉の句]] has the poem on its own, without a speech control.
