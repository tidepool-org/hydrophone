package testutil

import (
	"net/http"
	"sync/atomic"

	"github.com/gorilla/mux"
)

// WithRotatingVar provides a middleware to set variables on HTTP requests.
//
// It allows for bypassing a mux.Router with path parameters.
func WithRotatingVar(key string, ids []string) mux.MiddlewareFunc {
	maps := []map[string]string{}
	for _, id := range ids {
		maps = append(maps, map[string]string{key: id})
	}
	return WithRotatingVars(maps...)
}

// WithRotatingVars provides middleware to set variables on HTTP requests.
//
// The sets of vars are rotated through with each request. An atomic int is
// used to ensure thread-safety.
func WithRotatingVars(vars ...map[string]string) mux.MiddlewareFunc {
	i := &atomic.Int32{}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			v := vars[int(i.Add(1))%len(vars)]
			h.ServeHTTP(w, mux.SetURLVars(r, v))
		})
	}
}
