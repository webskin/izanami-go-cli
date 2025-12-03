package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestPoll_Success tests that Poll returns the token on 200 OK.
func TestPoll_Success(t *testing.T) {
	expectedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"

	// Create mock server that returns success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.URL.Path != "/api/admin/cli-token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("state") == "" {
			t.Error("state parameter missing")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"token": expectedToken})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	result, err := poller.Poll(context.Background())

	if err != nil {
		t.Fatalf("Poll() returned error: %v", err)
	}
	if !result.Ready {
		t.Error("Poll() returned Ready=false, want true")
	}
	if result.Token != expectedToken {
		t.Errorf("Poll() returned token %q, want %q", result.Token, expectedToken)
	}
}

// TestPoll_Pending tests that Poll returns Ready=false on 202 Accepted.
func TestPoll_Pending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "pending",
			"message": "Authentication pending",
		})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	result, err := poller.Poll(context.Background())

	if err != nil {
		t.Fatalf("Poll() returned error: %v", err)
	}
	if result.Ready {
		t.Error("Poll() returned Ready=true, want false")
	}
	if result.Token != "" {
		t.Errorf("Poll() returned token %q, want empty", result.Token)
	}
}

// TestPoll_NotFound tests that Poll returns error on 404 Not Found.
func TestPoll_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "CLI authentication state not found",
		})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	result, err := poller.Poll(context.Background())

	if err == nil {
		t.Fatal("Poll() returned nil error, want error")
	}
	if result != nil {
		t.Errorf("Poll() returned result %v, want nil", result)
	}
}

// TestPoll_Expired tests that Poll returns error on 410 Gone.
func TestPoll_Expired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "CLI authentication state has expired",
		})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	result, err := poller.Poll(context.Background())

	if err == nil {
		t.Fatal("Poll() returned nil error, want error")
	}
	if result != nil {
		t.Errorf("Poll() returned result %v, want nil", result)
	}
}

// TestPoll_RateLimited tests that Poll returns RetryAfter on 429.
func TestPoll_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":    "Too many requests",
			"retryAfter": 10,
		})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	result, err := poller.Poll(context.Background())

	if err != nil {
		t.Fatalf("Poll() returned error: %v", err)
	}
	if result.Ready {
		t.Error("Poll() returned Ready=true, want false")
	}
	if result.RetryAfter != 10 {
		t.Errorf("Poll() returned RetryAfter=%d, want 10", result.RetryAfter)
	}
}

// TestPoll_RateLimitedDefaultRetry tests default retry when retryAfter not in response.
func TestPoll_RateLimitedDefaultRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Too many requests",
		})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	result, err := poller.Poll(context.Background())

	if err != nil {
		t.Fatalf("Poll() returned error: %v", err)
	}
	if result.RetryAfter != 5 {
		t.Errorf("Poll() returned RetryAfter=%d, want 5 (default)", result.RetryAfter)
	}
}

// TestPoll_BadRequest tests that Poll returns error on 400 Bad Request.
func TestPoll_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Invalid state format",
		})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "invalid", 0)
	result, err := poller.Poll(context.Background())

	if err == nil {
		t.Fatal("Poll() returned nil error, want error")
	}
	if result != nil {
		t.Errorf("Poll() returned result %v, want nil", result)
	}
}

// TestPoll_ContextCancelled tests that Poll respects context cancellation.
func TestPoll_ContextCancelled(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 0)
	_, err := poller.Poll(ctx)

	if err == nil {
		t.Fatal("Poll() returned nil error, want context.Canceled")
	}
}

// TestWaitForToken_Success tests that WaitForToken returns token after polling.
func TestWaitForToken_Success(t *testing.T) {
	expectedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.success"
	var pollCount int32

	// Server returns pending for first 2 requests, then success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")

		if count < 3 {
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"token": expectedToken})
		}
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 50*time.Millisecond)
	token, err := poller.WaitForToken(context.Background(), 5*time.Second)

	if err != nil {
		t.Fatalf("WaitForToken() returned error: %v", err)
	}
	if token != expectedToken {
		t.Errorf("WaitForToken() returned token %q, want %q", token, expectedToken)
	}
	if pollCount < 3 {
		t.Errorf("WaitForToken() made %d poll requests, want at least 3", pollCount)
	}
}

// TestWaitForToken_Timeout tests that WaitForToken returns error on timeout.
func TestWaitForToken_Timeout(t *testing.T) {
	// Server always returns pending
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 50*time.Millisecond)
	_, err := poller.WaitForToken(context.Background(), 200*time.Millisecond)

	if err == nil {
		t.Fatal("WaitForToken() returned nil error, want timeout error")
	}
}

// TestWaitForToken_PermanentError tests that WaitForToken stops on permanent errors.
func TestWaitForToken_PermanentError(t *testing.T) {
	var pollCount int32

	// Server returns 404 after first pending
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")

		if count == 1 {
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
		}
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 50*time.Millisecond)
	_, err := poller.WaitForToken(context.Background(), 5*time.Second)

	if err == nil {
		t.Fatal("WaitForToken() returned nil error, want error")
	}
	// Should stop after 2 polls (pending, then error)
	if pollCount != 2 {
		t.Errorf("WaitForToken() made %d poll requests, want 2", pollCount)
	}
}

// TestWaitForToken_RateLimitHandling tests that WaitForToken respects retryAfter.
func TestWaitForToken_RateLimitHandling(t *testing.T) {
	expectedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ratelimit"
	var pollCount int32
	var timestamps []time.Time

	// Server returns rate limit on first request, then success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamps = append(timestamps, time.Now())
		count := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")

		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":    "rate limited",
				"retryAfter": 1, // 1 second wait
			})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"token": expectedToken})
		}
	}))
	defer server.Close()

	poller := NewTokenPoller(server.URL, "test-state-12345678901234567890123456789012", 50*time.Millisecond)
	token, err := poller.WaitForToken(context.Background(), 5*time.Second)

	if err != nil {
		t.Fatalf("WaitForToken() returned error: %v", err)
	}
	if token != expectedToken {
		t.Errorf("WaitForToken() returned token %q, want %q", token, expectedToken)
	}

	// Verify rate limiting was respected (at least 1 second between calls)
	if len(timestamps) >= 2 {
		gap := timestamps[1].Sub(timestamps[0])
		if gap < 900*time.Millisecond { // Allow some tolerance
			t.Errorf("WaitForToken() didn't respect retryAfter: gap was %v, want >= 1s", gap)
		}
	}
}

// TestCheckServerSupport_Supported tests detection of supported server.
func TestCheckServerSupport_Supported(t *testing.T) {
	// Server responds with 400 (invalid state format) - but endpoint exists
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/cli-login" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	supported := CheckServerSupport(context.Background(), server.URL)
	if !supported {
		t.Error("CheckServerSupport() returned false, want true")
	}
}

// TestCheckServerSupport_NotSupported tests detection of unsupported server.
func TestCheckServerSupport_NotSupported(t *testing.T) {
	// Server responds with 404 for cli-login endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	supported := CheckServerSupport(context.Background(), server.URL)
	if supported {
		t.Error("CheckServerSupport() returned true, want false")
	}
}

// TestCheckServerSupport_Redirect tests that redirect is treated as supported.
func TestCheckServerSupport_Redirect(t *testing.T) {
	// Server responds with redirect (302) - endpoint exists and works
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/cli-login" {
			http.Redirect(w, r, "https://oidc.example.com/auth", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	supported := CheckServerSupport(context.Background(), server.URL)
	if !supported {
		t.Error("CheckServerSupport() returned false for redirect, want true")
	}
}

// TestNewTokenPoller_DefaultInterval tests that 0 interval uses default.
func TestNewTokenPoller_DefaultInterval(t *testing.T) {
	poller := NewTokenPoller("http://example.com", "test-state", 0)
	if poller.pollInterval != DefaultPollInterval {
		t.Errorf("NewTokenPoller() with interval=0 set pollInterval=%v, want %v",
			poller.pollInterval, DefaultPollInterval)
	}
}

// TestNewTokenPoller_CustomInterval tests custom interval is respected.
func TestNewTokenPoller_CustomInterval(t *testing.T) {
	customInterval := 5 * time.Second
	poller := NewTokenPoller("http://example.com", "test-state", customInterval)
	if poller.pollInterval != customInterval {
		t.Errorf("NewTokenPoller() set pollInterval=%v, want %v",
			poller.pollInterval, customInterval)
	}
}

// TestNewTokenPoller_TrailingSlash tests that trailing slash is removed from URL.
func TestNewTokenPoller_TrailingSlash(t *testing.T) {
	poller := NewTokenPoller("http://example.com/", "test-state", 0)
	if poller.baseURL != "http://example.com" {
		t.Errorf("NewTokenPoller() set baseURL=%q, want %q",
			poller.baseURL, "http://example.com")
	}
}
