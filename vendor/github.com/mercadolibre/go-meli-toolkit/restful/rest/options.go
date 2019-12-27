package rest

import (
	"context"
	"net/http"
)

type reqOptions struct {
	ctx     context.Context
	headers http.Header
}

// Context returns the context.Context or a new background
// context from the given options object.
func (opt *reqOptions) Context() context.Context {
	if opt.ctx == nil {
		return context.Background()
	}
	return opt.ctx
}

// Headers returns the http.Header or a new empty header
// map with the given options headers.
func (opt *reqOptions) Headers() http.Header {
	return opt.headers
}

// Option function signature for decorating the reqOptions object.
type Option func(*reqOptions)

// Context injects the given context into the option.
func Context(ctx context.Context) Option {
	return func(opt *reqOptions) {
		opt.ctx = ctx
	}
}

// Headers injects the given headers into the option.
func Headers(headers http.Header) Option {
	return func(opt *reqOptions) {
		opt.headers = headers
	}
}
