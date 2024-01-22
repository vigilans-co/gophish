package ratelimit

import (
	log "github.com/gophish/gophish/logger"
	"net/http"
	"net/http/httptest"
	"testing"
)

var successHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("ok"))
	if err != nil {
		log.Error(err)
	}
})

func reachLimit(t *testing.T, handler http.Handler, limit int) {
	// Make `expected` requests and ensure that each return a successful
	// response.
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "127.0.0.1:"
	for i := 0; i < limit; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("no 200 on req %d got %d", i, w.Code)
		}
	}
	// Then, makes another request to ensure it returns the 429
	// status.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("no 429")
	}
}

func TestRateLimitEnforcement(t *testing.T) {
	expectedLimit := 3
	limiter := NewPostLimiter(WithRequestsPerMinute(expectedLimit))
	handler := limiter.Limit(successHandler)
	reachLimit(t, handler, expectedLimit)
}

func TestRateLimitCleanup(t *testing.T) {
	expectedLimit := 3
	limiter := NewPostLimiter(WithRequestsPerMinute(expectedLimit))
	handler := limiter.Limit(successHandler)
	reachLimit(t, handler, expectedLimit)

	// Set the timeout to be
	bucket, exists := limiter.visitors["127.0.0.1"]
	if !exists {
		t.Fatalf("doesn't exist for some reason")
	}
	bucket.lastSeen = bucket.lastSeen.Add(-limiter.expiry)
	limiter.Cleanup()
	_, exists = limiter.visitors["127.0.0.1"]
	if exists {
		t.Fatalf("exists for some reason")
	}
	reachLimit(t, handler, expectedLimit)
}
