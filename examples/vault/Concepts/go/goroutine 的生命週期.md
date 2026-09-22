---
title: goroutine 的生命週期
aliases: [goroutine lifetime]
type: concept
status: ready
domain: go
topics: [goroutine, 並行]
based_on: ["G01 goroutine 與等待"]
created: 2026-09-22
lang: zh-Hant
---

`go f()` 啟動 goroutine；`f` 返回時，這個 goroutine 才結束。呼叫它的函式返回，不會自動取消它。

若 `main` 返回，整個程式會結束，其他 goroutine 不會被等待。[語言規格](https://go.dev/ref/spec#Program_execution)

啟動工作前，先確定兩件事：

- 正常完成時，誰等待它？例如 `WaitGroup.Wait`。
- 結果不再需要時，它如何退出？例如監聽 `ctx.Done()`。

`WaitGroup` 只負責等待；取消必須由工作本身配合。
