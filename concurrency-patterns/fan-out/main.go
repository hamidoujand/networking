package main

import (
	"fmt"
	"sync"
)

func split(source <-chan int, n int) []<-chan int {
	dests := make([]<-chan int, 0, n)

	for range n {
		ch := make(chan int)
		dests = append(dests, ch)

		//create a goroutine that is going to do the send
		go func() {
			defer close(ch)

			for val := range source {
				ch <- val
			}

		}()
	}

	return dests
}

func main() {
	source := make(chan int)
	dests := split(source, 5)

	go func() {

		for i := range 10 {
			source <- i + 1
		}
		close(source)
	}()

	var wg sync.WaitGroup
	wg.Add(len(dests))

	for i, ch := range dests {
		go func() {
			defer wg.Done()
			for val := range ch {
				fmt.Printf("goroutine[%d]: %d\n", i+1, val)
			}
		}()
	}

	wg.Wait()
}
