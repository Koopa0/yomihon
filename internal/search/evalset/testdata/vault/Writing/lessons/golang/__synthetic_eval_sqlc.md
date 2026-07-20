---
title: 合成評測：sqlc 查詢
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-sqlc
---

## query.sql

sqlc 從具名 SQL 查詢生成 Go 型別與方法，使欄位與參數在編譯期接受檢查。SQL 仍是可審查的 canonical source。

## 套件邊界

查詢應放在擁有資料語意的 feature package，不集中成跨領域的全域資料庫層。
