---
title: 合成評測：Go slice 記憶體
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-memory
---

## backing array

slice 是底層陣列的一個視窗。複製 slice 只複製描述子，兩者仍可能別名到同一個 backing array，修改會互相可見。

## 所有權

跨邊界保存輸入時，若呼叫者仍可修改它，應以 slices.Clone 明確取得自己的資料。
