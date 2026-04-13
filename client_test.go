package quickbooks

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient creates a Client pointed at the given test server.
func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	ep, err := url.Parse(serverURL + "/v3/company/test-realm/")
	if err != nil {
		t.Fatal(err)
	}
	return &Client{
		Client:       http.DefaultClient,
		endpoint:     ep,
		minorVersion: "65",
		realmId:      "test-realm",
	}
}

func TestReq_429_NoRetries_ReturnsRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	var resp struct{}
	err := c.req("GET", "reports/ProfitAndLoss", nil, &resp, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
}

func TestReq_429_WithRetries_SucceedsOnSecondAttempt(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.SetMaxRetries(2)
	c.SetRetryDelay(10 * time.Millisecond)

	var resp map[string]string
	err := c.req("GET", "test", nil, &resp, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("resp = %v, want {status: ok}", resp)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected 2 calls, got %d", n)
	}
}

func TestReq_429_WithRetries_ExhaustsRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.SetMaxRetries(1)
	c.SetRetryDelay(10 * time.Millisecond)

	var resp struct{}
	err := c.req("GET", "test", nil, &resp, nil)

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected 2 calls (1 initial + 1 retry), got %d", n)
	}
}

func TestReq_429_RetrySleepsForConfiguredDelay(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.SetMaxRetries(1)
	c.SetRetryDelay(100 * time.Millisecond)

	start := time.Now()
	var resp map[string]string
	err := c.req("GET", "test", nil, &resp, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed < 80*time.Millisecond {
		t.Errorf("retry happened too fast (%v), expected ~100ms delay", elapsed)
	}
}

func TestReq_429_DefaultRetryDelayUsed(t *testing.T) {
	c := &Client{}
	if got := c.effectiveRetryDelay(); got != defaultRetryDelay {
		t.Errorf("effectiveRetryDelay() = %v, want %v", got, defaultRetryDelay)
	}
}

func TestReq_429_ConcurrentRequestsNoRace(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.SetMaxRetries(3)
	c.SetRetryDelay(10 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var resp map[string]string
			_ = c.req("GET", "test", nil, &resp, nil)
		}()
	}
	wg.Wait()
}

func TestSetMaxRetries_NegativeClampedToZero(t *testing.T) {
	c := &Client{}
	c.SetMaxRetries(-5)
	if c.maxRetries != 0 {
		t.Errorf("maxRetries = %d, want 0", c.maxRetries)
	}
}

func TestSetRetryDelay_NegativeClampedToZero(t *testing.T) {
	c := &Client{}
	c.SetRetryDelay(-1 * time.Second)
	if c.retryDelay != 0 {
		t.Errorf("retryDelay = %v, want 0", c.retryDelay)
	}
}

func TestDoWithThrottle_429_ReturnsRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	_, err := c.doWithThrottle(func() (*http.Request, error) {
		return http.NewRequest("GET", srv.URL+"/download/123", nil)
	})

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
}
