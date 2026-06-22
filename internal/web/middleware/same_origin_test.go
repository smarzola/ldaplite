package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSameOriginAllowsSafeMethodsWithoutOrigin(t *testing.T) {
	called := false
	handler := RequireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestRequireSameOriginRejectsMutatingRequestsWithoutOrigin(t *testing.T) {
	handler := RequireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/new", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRequireSameOriginRejectsCrossOriginMutatingRequests(t *testing.T) {
	logs := captureMiddlewareAuditLogs(t)
	handler := RequireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/new", nil)
	req.Header.Set("Origin", "https://evil.test")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	got := logs.String()
	assertAuditLogContains(t, got, `"event":"web.same_origin_denied"`)
	assertAuditLogContains(t, got, `"method":"POST"`)
	assertAuditLogContains(t, got, `"route":"/users/new"`)
	assertAuditLogContains(t, got, `"status":403`)
}

func TestRequireSameOriginAllowsSameOriginMutatingRequests(t *testing.T) {
	called := false
	handler := RequireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/new", nil)
	req.Header.Set("Origin", "https://ldaplite.test")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestRequireSameOriginUsesForwardedHost(t *testing.T) {
	called := false
	handler := RequireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/users/new", nil)
	req.Header.Set("X-Forwarded-Host", "ldaplite.example.com")
	req.Header.Set("Origin", "https://ldaplite.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}
