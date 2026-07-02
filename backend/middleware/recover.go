package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/SteVio89/stevio-home/apierr"
)

func Recover(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Printf("PANIC: %v\n%s", rec, debug.Stack())
					apierr.Write(w, apierr.ErrInternal())
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
