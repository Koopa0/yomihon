---
title: Note template
type: note
status: draft
domain: go
lang: en
---

A buffered channel can hold values before a receiver is ready:

```go
ch := make(chan int, 1)
ch <- 42
fmt.Println(<-ch) // 42
```

A second send before the receive would block: capacity is one, not unlimited.

Source: [Go specification — Channel types](https://go.dev/ref/spec#Channel_types).
