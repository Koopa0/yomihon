---
title: R06 讀一段 Go 的分支
aliases: [讀一段 Go 的分支]
type: lesson
status: ready
domain: yomihon
slug: reading-follow-a-go-branch
level: intermediate
topics: [Go, 閱讀, 名稱]
created: 2026-09-22
lang: zh-Hant
---

這是第二課的選讀支線。你不必先讀完整個專案；這次只追一個問題：查找名稱之後，程式怎麼處理候選檔案？

下面是可獨立執行的教學縮影。它保留「依候選數分支」的想法，省略真實程式的名稱正規化、索引建立與結果型別。不要把它複製回產品當成替代實作。

```go
package main

import "fmt"

func resolve(names map[string][]string, name string) string {
    members := names[name]
    switch len(members) {
    case 0:
        return "unresolved"
    case 1:
        return members[0]
    default:
        return "ambiguous"
    }
}

func main() {
    names := map[string][]string{
        "Plan":            {"Notes/alpha/Plan.md", "Notes/beta/Plan.md"},
        "Notes/alpha/Plan": {"Notes/alpha/Plan.md"},
    }
    for _, query := range []string{"Plan", "Notes/alpha/Plan", "Missing"} {
        fmt.Println(resolve(names, query))
    }
}
```

## 執行前，寫下三行輸出

第一個輸入為什麼不會回傳 alpha 的檔案？第三個輸入沒有出現在 map 裡，程式又是如何走到其中一個出口？

> [!question]- 沿著輸入走到 return
> 三行依序是 ambiguous、Notes/alpha/Plan.md、unresolved。
>
> 第一個名稱有兩個候選，進入 default，沒有「取第一個」這一步。第二個名稱只有一個候選。第三次查找得到 slice 型別的零值 nil；它的長度是零，所以走 case 0。
>
> 反例：若 map 明確存了一個空 slice，同樣走 case 0。這段程式不區分「鍵不存在」與「鍵存在但沒有候選」；若任務需要區分，就必須讀取 map 查找的第二個布林結果。這不是本次解析結果要回答的問題。

可把上面的程式碼存成暫存目錄內的 `main.go`，用 `go run main.go` 核對。範例的驗證工具鏈為 Go 1.27.0；map 零值與長度的規則可查 [Go 語言規格的 index expressions](https://go.dev/ref/spec#Index_expressions)。

## 再看真正的實作

[graph.go 的 Resolve](https://github.com/Koopa0/yomihon/blob/fab5e1c914fcb4da92a292772a05bf1984962ab4/internal/graph/graph.go#L111) 也有三個出口，但回傳的是帶有 Kind、RelPath 或 Candidates 的結構。模糊情況保留候選清單，讓後面的介面能交代它為什麼不選。

回頭看同一份檔案的索引建立，還能找到兩個這份縮影沒有做的工作：

- 檔名與相對路徑各有帶副檔名、不帶副檔名的鍵，aliases 也會加入。
- 新增與查找都使用同一個名稱正規化規則。

讀懂這個 switch 還不足以證明整個連結會成功。章節或區塊地址要在別處核對；[[名稱解析的三種結果#這份證據沒有回答的事]] 把邊界記了下來。

支線到此結束。回到 [[R02 追到宣告的地方]]，再沿主線進入 [[R03 用反例檢查理解]]。
