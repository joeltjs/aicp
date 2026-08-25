package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostAllowlist(t *testing.T) {
	const port = 3888
	handler := hostAllowlist(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), port)

	req := httptest.NewRequest("GET", "http://127.0.0.1:3888/api/checkpoints", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("loopback host should pass, got %d", rec.Code)
	}

	for _, host := range []string{"evil.com", "127.0.0.1:9999", "[::1]:3888x"} {
		req = httptest.NewRequest("GET", "http://127.0.0.1:3888/api/checkpoints", nil)
		req.Host = host
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("host %q should be forbidden, got %d", host, rec.Code)
		}
	}
}

func TestCsrfGuard(t *testing.T) {
	const port = 3888
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := csrfGuard(next, port)

	cross := httptest.NewRequest("POST", "http://127.0.0.1:3888/api/x", nil)
	cross.Header.Set("Origin", "http://attacker.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, cross)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin POST should be forbidden, got %d", rec.Code)
	}

	same := httptest.NewRequest("POST", "http://127.0.0.1:3888/api/x", nil)
	same.Header.Set("Origin", "http://127.0.0.1:3888")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, same)
	if rec.Code != http.StatusOK {
		t.Errorf("same-origin POST should pass, got %d", rec.Code)
	}

	noOrigin := httptest.NewRequest("POST", "http://127.0.0.1:3888/api/x", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, noOrigin)
	if rec.Code != http.StatusOK {
		t.Errorf("origin-less POST (curl) should pass, got %d", rec.Code)
	}
}
