---
type: concept
based_on: "[[Notes/alpha]]"
---

# Wide code

這篇筆記只為了量程式碼區塊的寬度：短的程式碼留在正文的寬度裡，一行放不下的才走到寬欄，而且不管視窗多寬、字多大，寬的區塊都不會壓到右邊的頁邊。

```go
x := 1
```

## 短的程式碼

一段短的程式碼，每一行都遠比正文窄。

```go
func main() {
	fmt.Println("short")
}
```

沒有標語言的程式碼：

```
make test
```

## 長的一行

```go
func RunSerializable(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
	}
	return nil
}
```

區塊下面的正文，回到正文的寬度。

## 中文註解

字數不多，但每個漢字都比程式碼的字母寬。

```go
// 每一輪等待都監聽上游的取消訊號不留下孤兒連線也不吞掉錯誤讓呼叫的人知道為什麼最後還是失敗了。
```

## 邊界

剛好放得下的一行：

```go
func boundary() {
	log.Println("the serializable transaction gave up after every attempt it.")
}
```

在最大的字級才放不下的一行：

```go
func larger() {
	log.Println("the serializable transaction gave up after every......")
}
```

字元不多，但縮排的 tab 讓它放不下：

```go
func indented() {
			log.Println("the serializable transaction gave up after every.......")
}
```

## 在提示框裡

> [!note] 提示框
> 提示框裡的程式碼留在提示框的寬度裡。
>
> ```go
> func RunSerializable(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
> ```

最後一段正文。
