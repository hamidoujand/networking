package main

import (
	"context"
	"fmt"
)

// SlowFunc represents an API that is slow but doesn't take ctx as argument to control it.
type SlowFunc func(string) (string, error)

type WithContext func(ctx context.Context, data string) (string, error)

func Timeout(slow SlowFunc) WithContext {
	return func(ctx context.Context, data string) (string, error) {
		resCh := make(chan string, 1)
		errCh := make(chan error, 1)

		//create a goroutine to handle the slow function call.
		go func() {
			res, err := slow(data)
			if err != nil {
				errCh <- fmt.Errorf("slow: %w", err)
				return
			}
			resCh <- res
		}()

		//control the goroutine
		select {
		case err := <-errCh:
			return "", err
		case res := <-resCh:
			return res, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
