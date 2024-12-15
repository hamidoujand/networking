package throttle

import (
	"context"
	"errors"
	"sync"
	"time"
)

//Throttle limits the frequency of a function call to some maximum number of
//invocations per unit of time

type Effector func(context.Context) (string, error)

func Throttle(effector Effector, max int, refill int, d time.Duration) Effector {
	var once sync.Once

	//Tracks the number of available tokens. Initially set to max, which is the maximum allowed calls at any time.
	var tokens = max

	return func(ctx context.Context) (string, error) {
		//This prevents unnecessary processing if the operation is already invalid.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		//the goroutine to refill tokens starts only once.
		//Even if multiple requests are made, this block runs only on the first call.
		once.Do(func() {
			//Generates periodic ticks every d duration to trigger the token refill logic.
			ticker := time.NewTicker(d)

			go func() {
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						//The tokens are refilled by adding refill tokens
						t := tokens + refill
						//If this exceeds max, the tokens are capped at max.
						if t > max {
							t = max
						}
						//assign refilled tokens
						tokens = t
					}
				}
			}()
		})
		//no token available, this fn call will return an error
		if tokens <= 0 {
			return "", errors.New("to many fn calls")
		}
		tokens--
		return effector(ctx)
	}
}
