package retry

import (
	"context"
	"log"
	"time"
)

// Effector The function that interacts with the service
type Effector func(ctx context.Context) (string, error)

func Retry(effector Effector, retries int, delay time.Duration) Effector {
	return func(ctx context.Context) (string, error) {
		for r := 0; ; r++ {
			response, err := effector(ctx)
			if err == nil || r >= retries {
				return response, err
			}
			log.Printf("attemp %d failed, retrying in %v\n", r+1, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
}
