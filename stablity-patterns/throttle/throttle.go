package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Effector func(ctx context.Context) (string, error)

func Throttle(effector Effector, max int, refill int, d time.Duration) Effector {
	// Tracks the number of available "slots" for calls. Initially set to the max value.
	// Each call to the throttled function decreases the token count.
	tokens := max
	// Ensures the refill logic (explained below) is initialized only once, even if the Throttle function is called multiple times.
	var once sync.Once

	return func(ctx context.Context) (string, error) {
		//refill logic
		once.Do(func() {
			ticker := time.NewTicker(d) //create a ticker one time only
			//so now every "d" the ticker will refill to tokens by a fixed amount

			//create a goroutine one time
			go func() {
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return //so the timer also gets cleaned.
					case <-ticker.C:
						//Add refill tokens to the current tokens.
						t := tokens + refill
						if t > max {
							//If tokens exceeds max, reset it to max to ensure we don’t exceed the maximum allowed calls.
							t = max
						}
						tokens = t
					}
				}
			}()
		})

		if tokens <= 0 {
			return "", errors.New("too many calls")
		}

		tokens--
		//do the call
		return effector(ctx)
	}
}

func exampleEffector(ctx context.Context) (string, error) {
	return "success", nil
}

func main() {
	withThrottle := Throttle(exampleEffector, 3, 1, time.Second)
	for range 5 {
		resp, err := withThrottle(context.Background())
		if err != nil {
			fmt.Println("Err:", err)
		} else {
			fmt.Println("Result:", resp)
		}

		time.Sleep(time.Millisecond * 300)
	}
}
