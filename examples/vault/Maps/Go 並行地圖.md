---
title: Go 並行地圖
type: moc
status: ready
map_kind: topic
created: 2026-09-22
lang: zh-Hant
---

## 啟動與等待

- [[G01 goroutine 與等待]] — 等待工作完成
- [[goroutine 的生命週期]] — 啟動後，何時結束？
- [[G05 固定數量的 worker]] — 限制同時處理的工作數

## 傳值與關閉

- [[G02 channel 的交接]] — 交付一個結果
- [[channel 的交接]] — 阻塞、緩衝與關閉
- [[G03 關閉 channel]] — 用 range 收完一串值

## 取消與收尾

- [[G04 取消不再需要的工作]] — 接收方提早離開
- [[取消與收尾]] — 通知與等待的差別
- [[提早返回的管線]] — 卡住的 send
- [[可以取消的管線]] — 讓 producer 退出
- [[G06 select 與逾時]] — 等結果，也等期限
