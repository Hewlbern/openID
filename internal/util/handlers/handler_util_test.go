//go:build cssstub

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerUtil_WithMiddleware(t *testing.T) {
	util := NewHandlerUtil()
	var calls []string

	middleware1 := func(next Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request) error {
			calls = append(calls, "middleware1")
			return next(w, r)
		}
	}

	middleware2 := func(next Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request) error {
			calls = append(calls, "middleware2")
			return next(w, r)
		}
	}

	handler := func(w http.ResponseWriter, r *http.Request) error {
		calls = append(calls, "handler")
		return nil
	}

	wrapped := util.WithMiddleware(handler, middleware1, middleware2)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	expected := []string{"middleware2", "middleware1", "handler"}
	if len(calls) != len(expected) {
		t.Errorf("WithMiddleware() calls = %v, want %v", calls, expected)
	}
	for i, call := range calls {
		if call != expected[i] {
			t.Errorf("WithMiddleware() calls[%d] = %v, want %v", i, call, expected[i])
		}
	}
}

func TestHandlerUtil_WithContext(t *testing.T) {
	util := NewHandlerUtil()
	ctx := context.WithValue(context.Background(), "key", "value")

	middleware := util.WithContext(ctx)
	handler := func(w http.ResponseWriter, r *http.Request) error {
		if r.Context().Value("key") != "value" {
			t.Errorf("WithContext() context value = %v, want %v", r.Context().Value("key"), "value")
		}
		return nil
	}

	wrapped := middleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
}

func TestHandlerUtil_WithTimeout(t *testing.T) {
	util := NewHandlerUtil()
	timeout := 100 * time.Millisecond

	middleware := util.WithTimeout(timeout)
	handler := func(w http.ResponseWriter, r *http.Request) error {
		select {
		case <-r.Context().Done():
			return r.Context().Err()
		case <-time.After(timeout + 50*time.Millisecond):
			return nil
		}
	}

	wrapped := middleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("WithTimeout() status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestHandlerUtil_WithRecovery(t *testing.T) {
	util := NewHandlerUtil()

	middleware := util.WithRecovery()
	handler := func(w http.ResponseWriter, r *http.Request) error {
		panic("test panic")
	}

	wrapped := middleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("WithRecovery() status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestHandlerUtil_WithCORS(t *testing.T) {
	util := NewHandlerUtil()

	middleware := util.WithCORS()
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}

	wrapped := middleware(handler)

	// Test OPTIONS request
	req := httptest.NewRequest("OPTIONS", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("WithCORS() Access-Control-Allow-Origin = %v, want %v", w.Header().Get("Access-Control-Allow-Origin"), "*")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT, DELETE, OPTIONS" {
		t.Errorf("WithCORS() Access-Control-Allow-Methods = %v, want %v", w.Header().Get("Access-Control-Allow-Methods"), "GET, POST, PUT, DELETE, OPTIONS")
	}
	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Errorf("WithCORS() Access-Control-Allow-Headers = %v, want %v", w.Header().Get("Access-Control-Allow-Headers"), "Content-Type, Authorization")
	}
	if w.Code != http.StatusOK {
		t.Errorf("WithCORS() status = %v, want %v", w.Code, http.StatusOK)
	}

	// Test regular request
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("WithCORS() Access-Control-Allow-Origin = %v, want %v", w.Header().Get("Access-Control-Allow-Origin"), "*")
	}
}

func TestHandlerUtil_WithErrorHandling(t *testing.T) {
	util := NewHandlerUtil()

	middleware := util.WithErrorHandling()
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return http.ErrAbortHandler
	}

	wrapped := middleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("WithErrorHandling() status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}
