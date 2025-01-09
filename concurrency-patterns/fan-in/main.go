package main

import (
	"fmt"
	"sync"
	"time"
)

func funnel(sources ...<-chan int) <-chan int {
	dest := make(chan int)
	var wg sync.WaitGroup

	wg.Add(len(sources))

	for _, ch := range sources {
		go func() {
			defer wg.Done()
			for n := range ch {
				dest <- n
			}
		}()
	}

	//another goroutine to close the chan
	go func() {
		wg.Wait()
		close(dest)
	}()

	return dest
}

func main() {
	sources := make([]<-chan int, 0)

	for range 3 {
		ch := make(chan int)
		sources = append(sources, ch)

		go func() {
			defer close(ch)
			for i := 1; i <= 5; i++ {
				ch <- i
				time.Sleep(time.Second)
			}
		}()

	}

	dest := funnel(sources...)
	for n := range dest {
		fmt.Println(n)
	}
}
