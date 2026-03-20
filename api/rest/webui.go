package rest

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// MountWebUI registers routes to serve the Vite/React production build from distDir.
// Serves GET / (index.html) and GET /assets/* from dist/assets. API routes registered
// on the same mux take precedence. The SPA uses hash-based routing (/#/...) so REST
// paths like /orgs are not claimed by the frontend router.
func MountWebUI(mux *http.ServeMux, distDir string) {
	if distDir == "" {
		return
	}
	abs, err := filepath.Abs(distDir)
	if err != nil {
		log.Printf("web UI: resolve dist path %q: %v", distDir, err)
		return
	}
	index := filepath.Join(abs, "index.html")
	if _, err := os.Stat(index); err != nil {
		log.Printf("web UI: skip mount (no %s): %v", index, err)
		return
	}

	assetsDir := filepath.Join(abs, "assets")
	assetsServer := http.FileServer(http.Dir(assetsDir))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", assetsServer))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, index)
	})
}
