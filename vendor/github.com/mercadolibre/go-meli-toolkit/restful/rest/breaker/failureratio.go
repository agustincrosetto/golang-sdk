package breaker

import "time"

const (
	defaultMinRequests = 50
	defaultRatio       = 0.2
)

func NewFailureRatioStrategy(maxRequests uint32, intervalSize, timeout time.Duration, minRequests uint32, ratio float64) CircuitBreaker {
	open := func(counts Counts) bool {
		currentRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= minRequests && currentRatio >= ratio
	}

	return newCircuitBreaker(maxRequests, intervalSize, timeout, open)
}

func DefaultBreakerFailureRatioStrategy(minRequests uint32, ratio float64) CircuitBreaker {
	return NewFailureRatioStrategy(defaultMaxRequests, defaultIntervalSize, defaultTimeout, minRequests, ratio)
}

func DefaultFailureRatioStrategy() CircuitBreaker {
	return DefaultBreakerFailureRatioStrategy(defaultMinRequests, defaultRatio)
}
