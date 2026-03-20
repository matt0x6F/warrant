package rest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMountWebUI_servesIndexAndAssets(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "index.html"), []byte("<!doctype html><html><body>ok</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(tmp, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "x.txt"), []byte("asset"), 0o644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	MountWebUI(mux, tmp)

	t.Run("index", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if body := rec.Body.String(); body == "" || body[0] != '<' {
			t.Fatalf("body %q", body)
		}
	})

	t.Run("assets", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/assets/x.txt", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if rec.Body.String() != "asset" {
			t.Fatalf("body %q", rec.Body.String())
		}
	})
}
