---
title: 合成評測：SQLite generation
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-sqlite
---

## 完整後切換

新的 SQLite generation 在 staging 中完成所有向量與驗證，才以單一交易原子切換 active manifest。建立期間舊 generation 繼續可讀。

## 保留範圍

切換後只保留 active 與 previous；derived store 不成為 vault 真相。
