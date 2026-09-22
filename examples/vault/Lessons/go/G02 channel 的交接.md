---
title: G02 channel 的交接
type: lesson
status: ready
domain: go
slug: go-channel-handoff
level: fundamental
topics: [Go, 並行, channel]
created: 2026-09-22
lang: zh-Hant
---

channel 讓 goroutine 傳遞值。`ch <- value` 發送，`value := <-ch` 接收；沒有緩衝的 channel 要等雙方都準備好，交接才會完成。[^channel]

這個程式把 `21` 交給另一個 goroutine，取回兩倍的結果：

```go
package main

import "fmt"

func twice(in <-chan int, out chan<- int) {
	n := <-in
	out <- n * 2
}

func main() {
	in := make(chan int)
	out := make(chan int)
	go twice(in, out)

	in <- 21
	fmt.Println(<-out)
}
```

`<-chan int` 只允許接收，`chan<- int` 只允許發送。參數把 `twice` 對兩端的用途寫進型別。輸出是 `42`。

## 去掉 go，還能算完嗎？

把 `go twice(in, out)` 改成 `twice(in, out)`，程式會停在哪裡？若只把 `in` 改成容量為一的 channel，能解決嗎？

> [!question]- 查看執行流程
> 普通函式呼叫會在 `n := <-in` 等待。`main` 尚未走到 `in <- 21`，又沒有其他發送者，因此無法繼續。
>
> 容量為一也沒有幫助：緩衝區仍是空的。容量提供存放值的位置，不會替你發送。

## 緩衝有容量

`make(chan int, 2)` 可以暫存兩個值。若沒有人接收，前兩次發送可以完成，第三次仍會等待。緩衝能吸收短暫的速度差，不能解決永久不接收的問題。

無緩衝 channel 的發送完成，只表示值已交出，不表示接收者已處理完結果。需要確認工作完成時，還要另設完成訊號或使用 WaitGroup：[[channel 的交接]]。

[^channel]: [Go 規格：Channel types](https://go.dev/ref/spec#Channel_types)、[Send statements](https://go.dev/ref/spec#Send_statements)。
