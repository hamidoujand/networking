package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Circuit represents the function that interacts with a resource.
type Circuit func(ctx context.Context) (string, error)

/*
Breaker is a circuit breaker implementation commonly used for backoff strategies
in resource-intensive operations like database connections or API calls. It protects
systems from cascading failures by temporarily "breaking" the circuit after a specified
number of consecutive failures and applying an exponential backoff strategy for retries.

### How It Works:

1. **Input Function (Circuit):**
   The `Breaker` function takes a `Circuit` as input. The `Circuit` is a function
   responsible for executing the actual resource-intensive operation (e.g., connecting
   to a database, making an HTTP request). The circuit breaker does not concern itself
   with the implementation details of this function; it only monitors its error output.

2. **State Tracking:**
   The breaker maintains two shared state variables:
   - `consecutiveFailures`: Tracks the number of consecutive errors returned by the circuit.
   - `lastAttempt`: Records the timestamp of the most recent call to the circuit.

   These variables persist within the returned closure, ensuring the state is shared across
   multiple calls to the returned function.

3. **Locks:**
   The breaker uses `sync.RWMutex` to manage access to the shared state. This ensures thread-safe
   behavior in concurrent scenarios, where multiple goroutines might invoke the returned function
   simultaneously. Locks are precautionary, and while they add minimal overhead, they are critical
   when the breaker is used in multi-goroutine environments.

4. **Threshold and Backoff:**
   - A failure threshold is defined (`failureThreshold`). If the number of consecutive failures
     reaches or exceeds this threshold, the circuit is "open" and rejects further requests.
   - When the circuit is open, the breaker calculates a retry time using exponential backoff:
     ```
     shouldRetryAt = lastAttempt.Add(time.Second * 2 << d)
     ```
     - Here, `d = consecutiveFailures - failureThreshold`.
     - The retry delay doubles for each failure beyond the threshold:
       - At `d = 0`: Retry delay = `2 seconds`.
       - At `d = 1`: Retry delay = `4 seconds`.
       - At `d = 2`: Retry delay = `8 seconds`, and so on.
   - The circuit remains open until the current time exceeds the calculated `shouldRetryAt`.

5. **Behavior on Circuit Call:**
   - If the circuit is closed (i.e., below the failure threshold), the input function (`circuit`) is executed.
   - If an error occurs, `consecutiveFailures` is incremented, and the error is returned.
   - If the call succeeds, `consecutiveFailures` is reset to zero, indicating the system has recovered.

6. **Why Use This Pattern?**
   - It protects systems from overloading during outages by reducing unnecessary retries.
   - It provides a gradual recovery mechanism using exponential backoff.
   - It ensures thread-safe behavior when the breaker is invoked concurrently.

### Example Walkthrough:

Assume `failureThreshold = 3`:
1. **First Call (Failure):**
   - `consecutiveFailures = 1`
   - Circuit is not open (`d = 1 - 3 = -2`), so the call proceeds.
   - Failure increments `consecutiveFailures` to `2`.

2. **Second Call (Failure):**
   - `consecutiveFailures = 2`
   - Circuit is still not open (`d = 2 - 3 = -1`), so the call proceeds.
   - Failure increments `consecutiveFailures` to `3`.

3. **Third Call (Failure):**
   - `consecutiveFailures = 3`
   - Circuit is now open (`d = 3 - 3 = 0`).
   - `shouldRetryAt = lastAttempt.Add(2 seconds)`.
   - If `time.Now()` is before `shouldRetryAt`, the call is rejected with `"service unreachable"`.

4. **Fourth Call (After Delay):**
   - If `time.Now()` exceeds `shouldRetryAt`, the call proceeds.
   - If it fails again, `consecutiveFailures = 4`, and `shouldRetryAt` is updated with a longer delay
     (`time.Second * 4`).

5. **Recovery:**
   - If the call eventually succeeds, `consecutiveFailures` is reset to zero, and the circuit closes.

This pattern is commonly used in distributed systems, microservices, and networked applications to
handle transient failures gracefully while avoiding cascading issues or system overload.
*/

func Breaker(circuit Circuit, failureThreshold int) Circuit {

	//Tracks the number of consecutive failures.
	consecutiveFailures := 0
	//Records the last time the circuit was attempted.
	lastAttempt := time.Now()
	//Both variables are shared across calls to the returned function. so we need to protect them.
	var m sync.RWMutex

	return func(ctx context.Context) (string, error) {
		m.RLock()
		//results in negative numbers, when it gets into positives then it means
		//we have reached the threshold so we calculate one last retry time.
		d := consecutiveFailures - failureThreshold
		if d >= 0 {
			//backoff is triggered.
			shouldRetryAt := lastAttempt.Add(time.Second * 2 << d)
			//If the current time is still within the cooling-off period, return a "service unavailable" error.
			if !time.Now().After(shouldRetryAt) {
				m.RUnlock()
				//still in cooling-off situation, no more request to service.
				return "", errors.New("service unreachable")
			}
			//else go ahead and make a request.
		}
		m.RUnlock()

		response, err := circuit(ctx)
		//we want to modify shared resources
		m.Lock()
		defer m.Unlock()
		lastAttempt = time.Now()
		//we have error, so we first inc the counter then return the response.
		if err != nil {
			consecutiveFailures++
			return response, err
		}
		//we do not have error, so we reset the counter and return the response.
		consecutiveFailures = 0
		return response, nil
	}
}
