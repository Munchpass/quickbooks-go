package quickbooks

import (
	"context"
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
	err := c.req(context.Background(), "GET", "reports/ProfitAndLoss", nil, &resp, nil)
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
	err := c.req(context.Background(), "GET", "test", nil, &resp, nil)
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
	err := c.req(context.Background(), "GET", "test", nil, &resp, nil)

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
	err := c.req(context.Background(), "GET", "test", nil, &resp, nil)
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
			_ = c.req(context.Background(), "GET", "test", nil, &resp, nil)
		}()
	}
	wg.Wait()
}

func TestSetMaxRetries_NegativeClampedToZero(t *testing.T) {
	c := &Client{}
	c.SetMaxRetries(-5)
	if c.maxRetries.Load() != 0 {
		t.Errorf("maxRetries = %d, want 0", c.maxRetries.Load())
	}
}

func TestSetRetryDelay_NegativeClampedToZero(t *testing.T) {
	c := &Client{}
	c.SetRetryDelay(-1 * time.Second)
	if c.retryDelayNs.Load() != 0 {
		t.Errorf("retryDelayNs = %v, want 0", c.retryDelayNs.Load())
	}
}

func TestDoWithThrottle_429_ReturnsRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	_, err := c.doWithThrottle(context.Background(), func() (*http.Request, error) {
		return http.NewRequest("GET", srv.URL+"/download/123", nil)
	})

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
}

func TestReq_429_ContextCancellationDuringRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.SetMaxRetries(3)
	c.SetRetryDelay(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	var resp struct{}
	err := c.req(ctx, "GET", "test", nil, &resp, nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	if elapsed > 1*time.Second {
		t.Errorf("should have cancelled quickly, took %v", elapsed)
	}

	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("expected 1 call before cancellation, got %d", n)
	}
}

func TestReq_429_ContextCancellationDuringThrottleWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.SetRetryDelay(5 * time.Second)

	// Trigger a throttle window by making one request.
	_ = c.req(context.Background(), "GET", "test", nil, nil, nil)

	// Now a second request should block in waitForThrottle. Cancel it quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := c.req(ctx, "GET", "test", nil, nil, nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	if elapsed > 1*time.Second {
		t.Errorf("should have cancelled quickly, took %v", elapsed)
	}
}
