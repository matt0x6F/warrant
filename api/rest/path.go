package rest

import "net/http"

// PathParam returns the path parameter from the request (Go 1.22+ PathValue).
func PathParam(r *http.Request, key string) string {
	return r.PathValue(key)
}
