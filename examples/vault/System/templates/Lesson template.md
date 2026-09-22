---
title: Lesson template
type: lesson
status: draft
domain: go
level: fundamental
slug: lesson-template
lang: en
---

## What this lesson covers

A buffered channel has finite capacity.

## What to do

What does this program print?

```go
package main

import "fmt"

func main() {
    ch := make(chan int, 1)
    ch <- 42
    fmt.Println(<-ch)
}
```

> [!question]- Result
> `42`. The buffer holds the value until the receive removes it. A second
> send before that receive would block.

## What it assumes

Functions and variables. See [[channel 的交接]] for send and receive rules.
