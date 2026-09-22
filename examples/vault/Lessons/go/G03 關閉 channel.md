---
title: G03 關閉 channel
type: lesson
status: ready
domain: go
slug: go-close-channel
level: fundamental
topics: [Go, 並行, channel]
created: 2026-09-22
lang: zh-Hant
---

收到一個值之後，接收者還需要知道：後面是否有下一個？`close(ch)` 宣告不會再發送；`for value := range ch` 會接收剩餘值，直到 channel 關閉且已取空。[^close]

```go
package main

import "fmt"

func squares() <-chan int {
	out := make(chan int, 3)
	go func() {
		defer close(out)
		for _, n := range []int{1, 2, 3} {
			out <- n * n
		}
	}()
	return out
}

func main() {
	values := squares()
	for value := range values {
		fmt.Println(value)
	}
	value, ok := <-values
	fmt.Println(value, ok)
}
```

## 關閉會丟掉尚未接收的值嗎？

假設 producer 在 `main` 開始接收前就完成了迴圈，`range` 還能拿到三個值嗎？最後一行又會印什麼？

> [!question]- 查看輸出與解析
> 三個值仍然在緩衝區。單一 producer 依序送入，所以輸出固定是：
>
> ```text
> 1
> 4
> 9
> 0 false
> ```
>
> 最後一次接收時，channel 已關閉且取空，得到 `int` 的零值與 `ok == false`。若收到的是 producer 真正送出的 `0`，`ok` 仍是 `true`。

## 誰能確定不會再發送？

本例只有一個 producer，由它在最後一次發送後關閉 `out`。接收者若自行關閉，producer 之後再發送就會 panic；重複關閉也會 panic。[^close]

拿掉 `defer close(out)`，前三個值仍會出現，但 `range` 會繼續等第四個值。若只有一次交接，雙方也知道次數，則未必需要關閉；關閉是結束訊號，不是每個 channel 都要執行的清理動作。

多個 producer 共用一個輸出時，必須先等全部停止發送，再由一處關閉。[[G05 固定數量的 worker]] 會用到這個安排。

[^close]: [Go 規格：Close](https://go.dev/ref/spec#Close)、[Receive operator](https://go.dev/ref/spec#Receive_operator) 與 [For statements with range clause](https://go.dev/ref/spec#For_statements_with_range_clause)。
