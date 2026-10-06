package idempotency

import (
	"bytes"
	"errors"
	"net/http"
	"time"
)

const HeaderIdempotencyKey = "Idempotency-Key"
const HeaderIdempotentReplayed = "Idempotent-Replayed"

// responseRecorder captures the status code, headers, and body of the response.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Middleware creates an HTTP middleware enforcing idempotency on mutating endpoints.
func Middleware(storage Storage, defaultTTL time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Idempotency applies to mutating requests (POST, PUT, PATCH)
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(HeaderIdempotencyKey)
			if key == "" {
				// If idempotency is strictly required, reject:
				// http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
				// For now, pass through if header is absent
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			result, err := storage.TryAcquire(ctx, key, defaultTTL)
			if err != nil {
				if errors.Is(err, ErrLocked) {
					w.Header().Set("Retry-After", "2")
					http.Error(w, "concurrent request in progress with this idempotency key", http.StatusConflict)
					return
				}
				http.Error(w, "idempotency check failure: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// If key was already processed, replay the cached response
			if result.Status == StatusCompleted && result.Response != nil {
				for k, v := range result.Response.Headers {
					w.Header().Set(k, v)
				}
				w.Header().Set(HeaderIdempotentReplayed, "true")
				w.WriteHeader(result.Response.StatusCode)
				_, _ = w.Write(result.Response.Body)
				return
			}

			// Record response
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			next.ServeHTTP(rec, r)

			// If status code is 5xx (server error), release lock to allow client retry
			if rec.statusCode >= 500 {
				_ = storage.Release(ctx, key)
				return
			}

			// Save successful/client responses to idempotency cache
			headers := make(map[string]string)
			for k, v := range rec.Header() {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}

			_ = storage.Complete(ctx, key, ResponseRecord{
				StatusCode: rec.statusCode,
				Headers:    headers,
				Body:       rec.body.Bytes(),
				CreatedAt:  time.Now(),
			}, defaultTTL)
		})
	}
}
