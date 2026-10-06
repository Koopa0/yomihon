package main

import "fmt"

func main() {
	ch := make(chan string, 1)
	ch <- "雪 & <tag>"

	fmt.Println(<-ch)
}
