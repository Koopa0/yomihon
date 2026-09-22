---
title: G06 select 與逾時
type: lesson
status: ready
domain: go
slug: go-select-timeout
level: intermediate
topics: [Go, 並行, select]
created: 2026-09-22
lang: zh-Hant
---

等待結果時，可以同時等三件事：值抵達、呼叫者取消、時間到。`select` 選擇一個可進行的分支；沒有分支就緒，也沒有 `default` 時，才會阻塞。[^select]

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func receive(ctx context.Context, values <-chan int, timeout time.Duration) (int, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case value, ok := <-values:
		if !ok {
			return 0, errors.New("channel 已關閉")
		}
		return value, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-timer.C:
		return 0, errors.New("等待逾時")
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	values := make(chan int)

	_, err := receive(ctx, values, 20*time.Millisecond)
	fmt.Println(err)
}
```

這次沒有發送者，也沒有人提早取消，所以會印出 `等待逾時`。計時器至少等指定時間才觸發，不保證函式恰好在第 20 毫秒返回。[^timer]

## 第一個 case 有優先權嗎？

把 `values` 改成容量為一，呼叫 `receive` 前先送入 `7`。如果值與計時器都已就緒，寫在最前面的接收分支是否一定勝出？再加一個 `default`，又會如何改變等待？

> [!question]- 查看分支選擇規則
> 多個分支就緒時，`select` 以均勻的偽隨機方式選一個，不保證取到值，也不保證逾時優先。把某個 `case` 移到最上面，不會提高它的優先權。
>
> `default` 只在沒有其他分支可進行時執行，而且立即執行。用它印出「逾時」，表示的其實是「此刻沒有結果」，並沒有等過指定時間。

## 逾時只結束這次等待

這個 `receive` 不會停止外部 producer。若結果來自另一個 goroutine，呼叫端還要取消它的 context，並等待它收尾，做法見 [[G04 取消不再需要的工作]]。需要讓多層函式共用同一個期限時，可用 [context.WithTimeout](https://pkg.go.dev/context#WithTimeout)，各層都接受並遵守該 context。

[^select]: [Go 規格：Select statements](https://go.dev/ref/spec#Select_statements)。
[^timer]: [time.NewTimer](https://pkg.go.dev/time#NewTimer)。
