---
title: G05 固定數量的 worker
type: lesson
status: ready
domain: go
slug: go-worker-pool
level: intermediate
topics: [Go, 並行, worker]
created: 2026-09-22
lang: zh-Hant
---

工作有一萬件，不必啟動一萬個 goroutine。固定三個 worker 從同一個 `jobs` channel 領取工作，便能把同時處理的工作數限制在三件。[^pool]

```go
package main

import (
	"fmt"
	"slices"
	"sync"
)

func worker(jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for n := range jobs {
		results <- n * n
	}
}

func main() {
	jobs := make(chan int)
	results := make(chan int)
	var wg sync.WaitGroup
	const workers = 3
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker(jobs, results, &wg)
	}

	go func() {
		defer close(jobs)
		for _, n := range []int{1, 2, 3, 4, 5, 6} {
			jobs <- n
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var values []int
	for value := range results {
		values = append(values, value)
	}
	slices.Sort(values)
	fmt.Println(values)
}
```

## 等待者應該放在哪裡？

若把 `wg.Wait()` 與 `close(results)` 移回 `main`，放在接收迴圈之前，會發生什麼事？

> [!question]- 查看執行流程
> worker 卡在 `results <- n*n`，`main` 卻在等 worker 結束。兩邊互等，無法前進。
>
> 原程式讓協調者等待，`main` 同時接收。全部 worker 結束後，協調者才關閉結果 channel，讓接收迴圈退出。

## 工作順序與結果順序

哪個 worker 拿到哪件工作、哪個結果先抵達，都不固定。`slices.Sort` 只整理最後輸出，不改變執行順序：

```text
[1 4 9 16 25 36]
```

這裡有三個 worker，另外還有送工作與等待收尾的 goroutine。「三個 worker」不等於整個程式只有三個 goroutine。

本例會收完全部結果。若只取一個就返回，送工作與送結果的地方都可能卡住，必須補上 [[G04 取消不再需要的工作]] 的退出方式。增加 channel 容量只能延後塞滿，不能取代取消。

[^pool]: [Go Blog：Pipelines and cancellation — Bounded parallelism](https://go.dev/blog/pipelines#bounded-parallelism)。
