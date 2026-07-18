---
title: 合成評測：Go context
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-context
---

## 取消傳播

Go 的 context 把逾時、取消訊號與請求範圍資料沿呼叫鏈向下傳遞。每個阻塞操作都應觀察 Done 或回傳的錯誤。

## 所有權

建立取消函式的一方負責呼叫它，避免背景工作與計時器滯留。
