---
title: G01 goroutine 與等待
type: lesson
status: ready
domain: go
slug: go-goroutine-wait
level: fundamental
topics: [Go, 並行, goroutine]
created: 2026-09-22
lang: zh-Hant
---

`go f()` 讓 `f` 在新的 goroutine 執行，呼叫者繼續往下走。它不會自動等待 `f` 完成。[^go]

先啟動一件工作，再用 `sync.WaitGroup` 等它結束：

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		fmt.Println("工作完成")
	}()

	wg.Wait()
	fmt.Println("main 結束")
}
```

## 哪一行會先印？

這兩行的順序是否固定？若移除 `wg.Wait()`，還能保證看見「工作完成」嗎？

> [!question]- 查看輸出與解析
> 輸出固定是：
>
> ```text
> 工作完成
> main 結束
> ```
>
> 工作先印字，返回時執行 `Done()`；計數歸零，`Wait()` 才返回。
>
> 移除 `Wait()` 後，工作可能尚未執行，`main` 就已返回。程式不會替其他 goroutine 等待。[^exit]

## 先登記，再啟動

這裡的 `Add(1)` 必須放在 `go` 前面。若移進 goroutine，`main` 可能先看見零計數，讓 `Wait()` 直接返回。`defer wg.Done()` 則把扣除計數放在工作返回時。[^wait]

WaitGroup 負責等待，不傳回計算結果，也不限制工作數量。把 `Wait()` 換成 `time.Sleep` 只能猜工作要多久，不能建立完成順序。每次啟動工作，都要安排它如何退出：[[goroutine 的生命週期]]。

[^go]: [Go 規格：Go statements](https://go.dev/ref/spec#Go_statements)。
[^exit]: [Go 規格：Program execution](https://go.dev/ref/spec#Program_execution)。
[^wait]: [sync.WaitGroup.Add](https://pkg.go.dev/sync#WaitGroup.Add)、[Done](https://pkg.go.dev/sync#WaitGroup.Done) 與 [Wait](https://pkg.go.dev/sync#WaitGroup.Wait)。
