package breaker

import (
	"fmt"
	"github.com/mercadolibre/go-meli-toolkit/godog"
	"sync"
	"time"
)

// circuitBreaker is a state machine to prevent sending requests that are likely to fail.
type circuitBreaker struct {
	// Target of the requests, for metrics
	targetId string

	// Number of requests allowed to pass through when the circuitBreaker is half-open
	maxRequests uint32

	// Cyclic period of the closed state for the circuitBreaker to clear the internal Counts
	intervalSize time.Duration

	// Period of the open state, after which the state of the circuitBreaker becomes half-open
	timeout time.Duration

	// Function to decide if the circuitBreaker should be opened after a request fail
	shouldOpen func(counts Counts) bool

	mutex    sync.Mutex
	state    State
	interval uint64
	counts   Counts
	expiry   time.Time
}

func newCircuitBreaker(maxRequests uint32, intervalSize, timeout time.Duration, shouldOpen strategy) *circuitBreaker {
	cb := new(circuitBreaker)

	if maxRequests == 0 {
		cb.maxRequests = defaultMaxRequests
	} else {
		cb.maxRequests = maxRequests
	}

	cb.intervalSize = intervalSize

	if timeout == 0 {
		cb.timeout = defaultTimeout
	} else {
		cb.timeout = timeout
	}

	if shouldOpen == nil {
		cb.shouldOpen = defaultShouldOpen
	} else {
		cb.shouldOpen = shouldOpen
	}

	cb.nextInterval(time.Now())

	return cb
}

// Set the target id
func (cb *circuitBreaker) SetTarget(targetId string) {
	cb.targetId = targetId
}

// State returns the current state of the circuitBreaker.
func (cb *circuitBreaker) State() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Allow checks if a new request can proceed. It returns a callback that should be used to
// register the success or failure in a separate step. If the circuit breaker doesn't allow
// requests, it returns an error.
func (cb *circuitBreaker) Allow() (done func(success bool), err error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	return func(success bool) {
		cb.afterRequest(generation, success)
	}, nil
}

func (cb *circuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		godog.RecordSimpleMetric("circuit_breaker.throughput.open.avoided", 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
		return generation, ErrOpenState
	} else if state == StateHalfOpen {
		if cb.counts.Requests >= cb.maxRequests {
			godog.RecordSimpleMetric("circuit_breaker.throughput.half_open.avoided", 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
			return generation, ErrTooManyRequests
		}
		godog.RecordSimpleMetric("circuit_breaker.throughput.half_open.allowed", 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
	} else {
		godog.RecordSimpleMetric("circuit_breaker.throughput.close.allowed", 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
	}

	cb.counts.onRequest()

	return generation, nil
}

func (cb *circuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if success {
		godog.RecordSimpleMetric("circuit_breaker.result.success", 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
	} else {
		godog.RecordSimpleMetric("circuit_breaker.result.failure", 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
	}

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *circuitBreaker) onSuccess(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.onSuccess()
	case StateHalfOpen:
		cb.counts.onSuccess()
		if cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
			cb.setState(StateClosed, now)
		}
	}
}

func (cb *circuitBreaker) onFailure(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.onFailure()
		if cb.shouldOpen(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

func (cb *circuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.nextInterval(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.interval
}

func (cb *circuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	cb.state = state
	cb.nextInterval(now)

	godog.RecordSimpleMetric(fmt.Sprintf("circuit_breaker.change_state.%s", state.String()), 1, new(godog.Tags).Add("target_id", cb.targetId).ToArray()...)
}

func (cb *circuitBreaker) nextInterval(now time.Time) {
	cb.interval++
	cb.counts.clear()

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.intervalSize == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.intervalSize)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}
