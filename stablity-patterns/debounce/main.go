package main

import (
	"context"
	"sync"
	"time"
)

/*
Debounce limits the frequency of a function invocation so that only the first or
last in a cluster of calls is actually performed.

we sometimes find ourselves performing a cluster of
potentially slow or costly operations where only one would do.

We’re all familiar with the
experience of using a search bar whose autocomplete pop-up doesn’t display
until after you pause typing

This pattern is similar to “Throttle”, in that it limits how often a function can be
called. But where Debounce restricts clusters of invocations, Throttle simply
limits according to time period.

*/

type Circuit func(ctx context.Context) (string, error)

// DebounceVersion1 It prevents rapid consecutive executions of circuit by enforcing a "cooldown period" (d) between calls.
func DebounceVersion1(circuit Circuit, d time.Duration) Circuit {
	//starts with the zero value (time.Time{}) and is updated every time the circuit runs.
	var threshold time.Time
	//caches the last successful result returned by the circuit function.
	var result string
	//caches the last error returned by the circuit function
	var err error
	var m sync.Mutex

	//Every call to this function will check if enough time (d) has passed since the last execution of circuit.
	return func(ctx context.Context) (string, error) {
		m.Lock()
		defer func() {
			//at the end we update the next threshold to be current time + d
			threshold = time.Now().Add(d)
			m.Unlock()
		}()

		//means the cooldown period (d) has not yet passed.
		if time.Now().Before(threshold) {
			//The function returns the cached values (result and err) from the last execution of circuit.
			return result, err
		}

		//cooldown period has passed
		result, err = circuit(ctx)
		//if it fails or succeeds we cache results and return them
		return result, err
	}
}
