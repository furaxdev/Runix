package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssueAndValidate(t *testing.T) {
	ts, err := NewTokenStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTokenStore: %v", err)
	}
	dev, err := ts.Issue("my-phone")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !ts.Valid(dev.Token) {
		t.Error("freshly issued token should be valid")
	}
	if ts.Valid("garbage") {
		t.Error("random string must not validate")
	}
}

func TestRevoke(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	dev, _ := ts.Issue("laptop")
	if err := ts.Revoke(dev.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if ts.Valid(dev.Token) {
		t.Error("revoked token must no longer validate")
	}
}

func TestMiddlewareBypassesLoopback(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("expected loopback request to bypass auth")
	}
}

func TestMiddlewareRejectsRemoteWithoutToken(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Error("expected remote request without a token to be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddlewareAcceptsRemoteWithValidToken(t *testing.T) {
	ts, _ := NewTokenStore(t.TempDir())
	ts.LocalBypass = true
	dev, _ := ts.Issue("phone")
	called := false
	h := ts.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest("GET", "/api/tags", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	req.Header.Set("Authorization", "Bearer "+dev.Token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("expected remote request with a valid token to pass")
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:1234": true,
		"[::1]:1234":     true,
		"10.0.0.5:1234":  false,
	}
	for addr, want := range cases {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}
