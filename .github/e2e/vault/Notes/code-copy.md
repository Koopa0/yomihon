---

type: concept
based_on: "[[Notes/alpha]]"

---

# Code copy

```go
package main

import "fmt"

func main() {
	ch := make(chan string, 1)
	ch <- "雪 & <tag>"

	fmt.Println(<-ch)
}
```

```text
plain	<-

last line
```

![[Notes/code-copy-embedded]]
