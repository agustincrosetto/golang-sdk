package retry

import (
	"net/http"
	"strconv"
	"time"
)

// Simple Retry Strategy
type simpleRetryStrategy struct {
	maxRetries     int
	delay          time.Duration
	allowedMethods []string
}

func NewSimpleRetryStrategy(maxRetries int, delay time.Duration, verbs ...string) RetryStrategy {
	if maxRetries < 0 || delay < 0 {
		return nil
	}
	allowedMethods := defaultRetriableMethods
	if len(verbs) > 0 {
		allowedMethods = verbs
	}
	return simpleRetryStrategy{maxRetries, delay, allowedMethods}
}

func (r simpleRetryStrategy) ShouldRetry(req *http.Request, resp *http.Response, err error, retries int) RetryResponse {
	retry := retries < r.maxRetries && (err != nil || resp.StatusCode >= http.StatusInternalServerError) && isMethodAllowed(req.Method, r.allowedMethods)
	return &retryResponse{retry, r.delay}
}

func (r simpleRetryStrategy) GetParams() map[string]interface{} {

	return map[string]interface{}{
		"max_retries": r.maxRetries,
		"delay":       strconv.Itoa(int(r.delay.Seconds() * 1000)),
	}
}

func (r simpleRetryStrategy) Clone(verbs ...string) RetryStrategy {
	return NewSimpleRetryStrategy(r.maxRetries, r.delay, verbs...)
}