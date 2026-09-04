---
title: L03 Mark a paragraph to be read aloud
type: lesson
status: ready
domain: japanese
slug: l03-read-aloud
level: intermediate
created: 2026-04-20
lang: en
---

A note that declares `lang: ja` keeps its language while the interface around it
changes. To give one of its paragraphs a control that speaks it, put this
comment on a line of its own, immediately above the paragraph:

```
<!-- read-aloud: ja -->
```

The paragraph below carries one:

<!-- read-aloud: ja -->
<ruby>古池<rt>ふるいけ</rt></ruby>や<ruby>蛙<rt>かわず</rt></ruby><ruby>飛<rt>と</rt></ruby>びこむ<ruby>水<rt>みず</rt></ruby>の<ruby>音<rt>おと</rt></ruby>

The readings are taken out before the words are spoken, so the furigana is not
read out beside them. `ja` is the only language the marker takes.

> [!tip]- Where the marker does nothing
> The control is added to a lesson the contract governs, and to nothing else.
> On any other note the comment stays a comment and the page says nothing about
> it — so a marker that seems to do nothing is usually on a note whose `type` is
> not `lesson`.

[[芭蕉の句]] has the poem on its own, without the apparatus.
