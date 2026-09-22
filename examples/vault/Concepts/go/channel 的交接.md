---
title: channel 的交接
aliases: [channel send and receive]
type: concept
status: ready
domain: go
topics: [channel, 同步]
based_on: ["G02 channel 的交接", "G03 關閉 channel"]
created: 2026-09-22
lang: zh-Hant
---

| 操作 | 何時能繼續 |
| --- | --- |
| 無緩衝 channel 的 send | 有接收方配對時 |
| 有緩衝 channel 的 send | 緩衝區有空位時 |
| receive | 有值可取，或 channel 已關閉時 |
| 向 nil channel send／receive | 永遠阻塞 |

`close(ch)` 表示不再送值。已緩衝的值仍可取出；取完後，`v, ok := <-ch` 得到零值與 `false`。向已關閉的 channel 送值會 panic。

關閉前，必須確定所有 send 都已結束。單一 producer 可以自行關閉；多個 worker 共用輸出時，由等待所有 worker 的協調者關閉。

[Go 規格：Channel types](https://go.dev/ref/spec#Channel_types)、[Close](https://go.dev/ref/spec#Close)
