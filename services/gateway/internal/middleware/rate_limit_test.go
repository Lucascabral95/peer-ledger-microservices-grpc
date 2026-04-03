package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDecide_ExemptPath(t *testing.T) {
	rl := NewRateLimiter(
		Policy{Name: "default", Requests: 2, Window: time.Minute},
		nil,
		[]string{"/health"},
		time.Minute,
		false,
		func() time.Time { return time.Unix(0, 0) },
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	_, exempt := rl.Decide(req)
	if !exempt {
		t.Fatalf("expected exempt path")
	}
}

func TestDecide_RouteSpecificPolicy(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(
		Policy{Name: "default", Requests: 10, Window: time.Minute},
		map[string]Policy{
			"/transfers": {Name: "transfers", Requests: 1, Window: time.Minute},
		},
		nil,
		time.Minute,
		false,
		func() time.Time { return now },
	)

	req := httptest.NewRequest(http.MethodPost, "/transfers", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	decision, exempt := rl.Decide(req)
	if exempt || !decision.Allowed {
		t.Fatalf("expected first transfer allowed")
	}

	decision, exempt = rl.Decide(req)
	if exempt || decision.Allowed {
		t.Fatalf("expected second transfer blocked")
	}
	if decision.PolicyName != "transfers" {
		t.Fatalf("expected transfers policy, got %q", decision.PolicyName)
	}
}

func TestMiddleware_SetsHeadersAndBlocks(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(
		Policy{Name: "default", Requests: 1, Window: time.Minute},
		nil,
		nil,
		time.Minute,
		false,
		func() time.Time { return now },
	)

	onLimit := func(w http.ResponseWriter, r *http.Request, d Decision) {
		w.WriteHeader(http.StatusTooManyRequests)
	}

	handler := rl.Middleware(onLimit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/users/user-001", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr1.Code)
	}
	if rr1.Header().Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("expected X-RateLimit-Limit header")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/users/user-001", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestClientIP_TrustProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:1234"

	got := clientIP(req, true)
	if got != "203.0.113.10" {
		t.Fatalf("expected forwarded ip, got %q", got)
	}
}
