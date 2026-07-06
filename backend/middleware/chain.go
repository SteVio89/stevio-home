package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Chain wraps a handler with middleware, applied left-to-right.
// Chain(handler, A, B, C) executes as: A(B(C(handler)))
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
