package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_SetsContextAndHeader(t *testing.T) {
	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if capturedID == "" {
		t.Error("expected request ID in context")
	}
	if w.Header().Get(requestIDHeader) != capturedID {
		t.Errorf("header %s = %q, context = %q", requestIDHeader, w.Header().Get(requestIDHeader), capturedID)
	}
}

func TestRequestID_UsesIncomingHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id != "custom-id" {
			t.Errorf("got %q", id)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, "custom-id")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Header().Get(requestIDHeader) != "custom-id" {
		t.Errorf("header = %q", w.Header().Get(requestIDHeader))
	}
}

func TestRecoverer_RecoversFromPanic(t *testing.T) {
	handler := Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d", w.Code)
	}
	if w.Body.String() != `{"error":"internal server error","code":"internal","retriable":false}` {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestRecoverer_PassesThrough(t *testing.T) {
	handler := Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got status %d", w.Code)
	}
}
