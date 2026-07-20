---
title: 合成評測：Go interface
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-interface
---

## 消費端介面

Go 介面通常由需要行為的消費端定義，並保持方法集合很小。生產端回傳具體型別，讓抽象從真實需求中被發現。

## 命名

單一方法介面常以方法語意加 er 命名，不為測試預先製造介面。
