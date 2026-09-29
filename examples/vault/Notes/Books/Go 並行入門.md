---
title: Go 並行入門
type: study-path
status: ready
topics: [Go, 並行, goroutine, channel]
created: 2026-09-22
lang: zh-Hant
---

# Go 並行入門

從一個 goroutine 開始，把整數交給另一份工作處理，再逐步加入關閉、取消與固定數量的 worker。

適合已會 Go 的函式、slice 與 `for` 迴圈，尚未寫過並行程式的讀者。五課主線各有一支完整程式；`select` 與逾時放在第四課的選讀支線。

範例以 Go 1.27.0 驗證，只使用標準函式庫。每次取一課的程式，存成獨立目錄中的 `main.go`，執行 `go run main.go`；各課都有自己的 `main`，不要合併在同一個檔案。

## 啟動與交接 {sequence=primary}

- [[G01 goroutine 與等待]] — 啟動工作，等它完成
- [[G02 channel 的交接]] — 發送值，取回結果
- [[G03 關閉 channel]] — 告訴接收者不會再有值

## 取消與限制工作數量 {sequence=primary}

- [[G04 取消不再需要的工作]] — 接收者提早離開，producer 也能退出
    - 要限制等待時間時 {sequence=local}
        - [[G06 select 與逾時]] — 等待值、取消或計時器
- [[G05 固定數量的 worker]] — 限制同時處理的工作數

## 查閱與對照 {sequence=none}

- [[Go 並行地圖]] — 等待、交接與收尾的關係
- [[提早返回的管線]] — 找出失去接收者的發送點
- [[可以取消的管線]] — 比較加入取消後的退出路徑
