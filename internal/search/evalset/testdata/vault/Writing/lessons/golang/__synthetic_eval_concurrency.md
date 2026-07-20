---
title: 合成評測：Go 並行生命週期
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-concurrency
---

## goroutine 生命週期

啟動 goroutine 前必須知道它如何停止、誰等待它，以及錯誤如何回傳。無主的 goroutine 會造成資源洩漏。

## channel

關閉 channel 的責任通常屬於送出端；接收端不應猜測其他送出者是否仍存在。
