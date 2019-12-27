package breaker

import (
	"time"
)

const (
	defaultMaxFailures         = 50
	defaultMaxFailuresInterval = time.Hour
)

func NewConsecutiveFailuresStrategy(maxRequests uint32, intervalSize, timeout time.Duration, maxFailures uint32) CircuitBreaker {
	open := func(counts Counts) bool {
		return counts.ConsecutiveFailures >= maxFailures
	}

	return newCircuitBreaker(maxRequests, intervalSize, timeout, open)
}

func DefaultBreakerConsecutiveFailuresStrategy(maxFailures uint32) CircuitBreaker {
	return NewConsecutiveFailuresStrategy(defaultMaxRequests, defaultMaxFailuresInterval, defaultTimeout, maxFailures)
}

func DefaultConsecutiveFailuresStrategy() CircuitBreaker {
	return DefaultBreakerConsecutiveFailuresStrategy(defaultMaxFailures)
}
