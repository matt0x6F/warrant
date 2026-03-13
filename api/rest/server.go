package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns the chi router with global middleware and routes.
// Any extra middleware is applied before the first route (chi requires middleware before routes).
func NewRouter(extraMiddleware ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	for _, mw := range extraMiddleware {
		if mw != nil {
			r.Use(mw)
		}
	}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Warrant metrics\n# Expose Prometheus or other metrics here when needed.\n"))
	})

	return r
}
