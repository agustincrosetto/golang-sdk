package breaker

import "fmt"

// State is a type that represents a state of circuitBreaker.
type State int

// These constants are states of circuitBreaker.
const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String implements stringer interface.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}
