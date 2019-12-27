package breaker

import (
	"errors"
	"time"
)

const (
	defaultMaxRequests  = 50
	defaultIntervalSize = 200 * time.Millisecond
	defaultTimeout      = time.Minute
)

var (
	// ErrTooManyRequests is returned when the CB state is half open and the requests count is over the cb maxRequests
	ErrTooManyRequests = errors.New("too many requests")
	// ErrOpenState is returned when the CB state is open
	ErrOpenState = errors.New("circuit breaker is open")
)

func defaultShouldOpen(counts Counts) bool {
	return counts != counts // false
}

type strategy func(counts Counts) bool

type CircuitBreaker interface {
	SetTarget(targetId string)
	State() State
	Allow() (done func(success bool), err error)
}
