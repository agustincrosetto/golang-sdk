package tracing

import (
	"context"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
)

// tracingKey type is an internal type used for
// assigning values to context.Context in a way that
// only this package is able to access.
// Example:
//   ctx := context.WithValue(context.Background(), tracingKey, "value")
//
//   ctx.Value(rqCtxKey) // Read previous saved value from context
type tracingKey struct{}

const (
	// RequestIDHeaderHTTP exposes the header to use for reading
	// and propagating the request id from an HTTP context.
	RequestIDHeaderHTTP = "x-request-id"

	// RequestFlowStarterHeaderHTTP is the HTTP header that the tracing
	// library forwards when the application start a new request flow.
	RequestFlowStarterHeaderHTTP = "x-flow-starter"

	// ForwardedHeadersNameHTTP is the HTTP header that contains the comma
	// separated value of request headers that must be forwarded to the
	// outgoing HTTP request that the application performs.
	ForwardedHeadersNameHTTP = "x-forwarded-header-names"
)

// ContextFromRequest given a http.Request returns a context decorated with the
// headers from the request that must be forwarded by the application in http
// requests to external services.
func ContextFromRequest(req *http.Request) context.Context {
	headers := http.Header{}

	// Read the header that lists all headers to be forwarded and add
	// them to the headers map.
	forwardedHeaders := strings.Split(req.Header.Get(ForwardedHeadersNameHTTP), ",")
	for _, header := range forwardedHeaders {
		key := strings.TrimSpace(header)
		if value := req.Header.Get(key); value != "" {
			headers.Set(key, value)
		}
	}

	// Check to see if x-request-id is forwarded from the request. If not
	// generate a new request id and assign to the the header bag.
	if reqID := headers.Get(RequestIDHeaderHTTP); reqID == "" {
		headers.Set(RequestIDHeaderHTTP, newRequestID())
	}

	return context.WithValue(req.Context(), tracingKey{}, headers)
}

// ForwardedHeaders returns the headers that must be forwarded by HTTP clients
// given a request context.Context.
func ForwardedHeaders(ctx context.Context) http.Header {
	headers, _ := ctx.Value(tracingKey{}).(http.Header)
	return headers
}

// RequestID returns the request id given a context.
// If the context does not contain a requestID, then
// an empty string is returned.
func RequestID(ctx context.Context) string {
	headers := ForwardedHeaders(ctx)
	return headers.Get(RequestIDHeaderHTTP)
}

// NewFlowStarterContext decorates the given context with a
// request id and marks it as an internal request.
func NewFlowStarterContext(ctx context.Context) context.Context {
	headers := http.Header{}

	headers.Add(RequestIDHeaderHTTP, newRequestID())
	headers.Add(RequestFlowStarterHeaderHTTP, "true")

	return context.WithValue(ctx, tracingKey{}, headers)
}

// newRequestID generates a new UUIDv4. If generation fails, which
// could happen if the randomness source is depleted, then it
// returns an empty string.
func newRequestID() string {
	uuid, err := uuid.NewV4()
	if err != nil {
		return ""
	}
	return uuid.String()

}
