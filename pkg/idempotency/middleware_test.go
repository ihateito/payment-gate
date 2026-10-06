package idempotency

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdempotencyMiddleware_Replay(t *testing.T) {
	storage := NewMemoryStorage()
	var execCount int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&execCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"pay_123","status":"SUCCEEDED"}`))
	})

	middleware := Middleware(storage, 1*time.Minute)(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/v1/charges", nil)
	req1.Header.Set(HeaderIdempotencyKey, "idemp-unique-1")
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec1.Code)
	}

	body1, _ := io.ReadAll(rec1.Body)

	// Second request with SAME idempotency key
	req2 := httptest.NewRequest(http.MethodPost, "/v1/charges", nil)
	req2.Header.Set(HeaderIdempotencyKey, "idemp-unique-1")
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec2.Code)
	}
	if rec2.Header().Get(HeaderIdempotentReplayed) != "true" {
		t.Errorf("expected Idempotent-Replayed header")
	}

	body2, _ := io.ReadAll(rec2.Body)
	if string(body1) != string(body2) {
		t.Errorf("bodies mismatch: %s vs %s", string(body1), string(body2))
	}

	if atomic.LoadInt64(&execCount) != 1 {
		t.Errorf("expected handler executed exactly 1 time, got %d", execCount)
	}
}

func TestIdempotencyMiddleware_ConcurrentConflict(t *testing.T) {
	storage := NewMemoryStorage()
	gate := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-gate // wait until signal
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"done"}`))
	})

	middleware := Middleware(storage, 1*time.Minute)(handler)

	var wg sync.WaitGroup
	var code1, code2 int

	wg.Add(2)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodPost, "/v1/charges", nil)
		req.Header.Set(HeaderIdempotencyKey, "concurrent-key")
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
		code1 = rec.Code
	}()

	// Short pause to ensure request 1 starts first
	time.Sleep(10 * time.Millisecond)

	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodPost, "/v1/charges", nil)
		req.Header.Set(HeaderIdempotencyKey, "concurrent-key")
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
		code2 = rec.Code
	}()

	// Release first handler
	time.Sleep(30 * time.Millisecond)
	close(gate)
	wg.Wait()

	if (code1 == http.StatusOK && code2 == http.StatusConflict) ||
		(code1 == http.StatusConflict && code2 == http.StatusOK) {
		// As expected, one completed and one got 409 Conflict
	} else {
		t.Errorf("expected one 200 and one 409, got %d and %d", code1, code2)
	}
}
