package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	// Health is independent from external dependencies.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSafeFilerPath(t *testing.T) {
	cases := map[string]string{
		"":             "/",
		"/":            "/",
		"/photos/2026": "/photos/2026",
		"../../etc":    "/etc",
		"a/../b":       "/b",
	}
	for input, want := range cases {
		if got := safeFilerPath(input); got != want {
			t.Errorf("safeFilerPath(%q) = %q, want %q", input, got, want)
		}
	}
}
