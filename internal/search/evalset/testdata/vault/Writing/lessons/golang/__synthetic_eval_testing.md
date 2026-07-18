---
title: 合成評測：Go watched-red 測試
aliases: []
type: lesson
domain: golang
topics: [go-basics]
status: draft
created: 2026-01-01
updated: 2026-01-01
slug: synthetic-go-testing
---

## RED の證據

防線測試必須先以可編譯的行為 mutation 證明會失敗，並確認失敗原因正是要守的契約。只看綠燈不能證明測試有效。

## Oracle

預期值應獨立於被測實作，wire 契約使用字面 bytes，而不是拿程式自己的常數和自己比較。
