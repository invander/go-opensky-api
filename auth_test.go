package opensky

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestTokenServer(t *testing.T, token string, expiresIn int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("expected Content-Type application/x-www-form-urlencoded, got %s", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %s", r.FormValue("grant_type"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: token,
			ExpiresIn:   expiresIn,
			TokenType:   "Bearer",
		})
	}))
}

func TestTokenManager_GetToken(t *testing.T) {
	t.Parallel()

	srv := newTestTokenServer(t, "test-token-abc", 1800)
	defer srv.Close()

	tm := &tokenManager{
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   srv.Client(),
		tokenURL:     srv.URL,
	}

	token, err := tm.getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-abc" {
		t.Errorf("expected token 'test-token-abc', got %q", token)
	}
}

func TestTokenManager_CachesToken(t *testing.T) {
	t.Parallel()

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "cached-token",
			ExpiresIn:   1800,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	tm := &tokenManager{
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   srv.Client(),
		tokenURL:     srv.URL,
	}

	for i := 0; i < 5; i++ {
		token, err := tm.getToken()
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if token != "cached-token" {
			t.Errorf("call %d: expected 'cached-token', got %q", i, token)
		}
	}

	if c := atomic.LoadInt32(&callCount); c != 1 {
		t.Errorf("expected 1 HTTP call (token cached), got %d", c)
	}
}

func TestTokenManager_RefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: fmt.Sprintf("token-%d", n),
			ExpiresIn:   1800,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	tm := &tokenManager{
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   srv.Client(),
		tokenURL:     srv.URL,
	}

	token1, err := tm.getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 != "token-1" {
		t.Errorf("expected 'token-1', got %q", token1)
	}

	// Simulate token expiry.
	tm.expiresAt = time.Now().Add(-1 * time.Second)

	token2, err := tm.getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token2 != "token-2" {
		t.Errorf("expected 'token-2', got %q", token2)
	}

	if c := atomic.LoadInt32(&callCount); c != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", c)
	}
}

func TestTokenManager_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tm := &tokenManager{
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   srv.Client(),
		tokenURL:     srv.URL,
	}

	_, err := tm.getToken()
	if err == nil {
		t.Fatal("expected error for server 500, got nil")
	}
}

func TestTokenManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "concurrent-token",
			ExpiresIn:   1800,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	tm := &tokenManager{
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   srv.Client(),
		tokenURL:     srv.URL,
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := tm.getToken()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if token != "concurrent-token" {
				t.Errorf("expected 'concurrent-token', got %q", token)
			}
		}()
	}
	wg.Wait()

	if c := atomic.LoadInt32(&callCount); c != 1 {
		t.Errorf("expected 1 HTTP call with concurrent access, got %d", c)
	}
}
