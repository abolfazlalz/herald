package main

import (
	"fmt"
	"time"
)

func main() {
	nameCh := make(chan string)
	name := "Aba"
	go func() {
		for {
			nameCh <- fmt.Sprintf("Hello, %s!", name)
			time.Sleep(time.Second * 5)
		}
	}()

	// for {
	// 	select {
	// 	test := <-nameCh
	// 	fmt.Println(test)
	// }
	// }
}
