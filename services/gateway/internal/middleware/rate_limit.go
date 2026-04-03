package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Policy struct {
	Name     string
	Requests int
	Window   time.Duration
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Limit      int
	Remaining  int
	ResetAfter time.Duration
	ClientKey  string
	PolicyName string
}

type bucketEntry struct {
	Tokens     float64
	LastRefill time.Time
}

type RateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*bucketEntry
	defaultPolicy   Policy
	routePolicies   map[string]Policy
	exemptPaths     map[string]struct{}
	trustProxy      bool
	cleanupInterval time.Duration
	lastCleanup     time.Time
	now             func() time.Time
}

func NewRateLimiter(defaultPolicy Policy, routePolicies map[string]Policy, exemptPaths []string, cleanupInterval time.Duration, trustProxy bool, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}

	exempt := make(map[string]struct{}, len(exemptPaths))
	for _, path := range exemptPaths {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			exempt[trimmed] = struct{}{}
		}
	}

	policiesCopy := make(map[string]Policy, len(routePolicies))
	for path, policy := range routePolicies {
		policiesCopy[path] = policy
	}

	return &RateLimiter{
		buckets:         make(map[string]*bucketEntry),
		defaultPolicy:   defaultPolicy,
		routePolicies:   policiesCopy,
		exemptPaths:     exempt,
		trustProxy:      trustProxy,
		cleanupInterval: cleanupInterval,
		now:             now,
	}
}

func (r *RateLimiter) Middleware(onLimit func(http.ResponseWriter, *http.Request, Decision)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			decision, exempt := r.Decide(req)
			if exempt {
				next.ServeHTTP(w, req)
				return
			}

			setRateLimitHeaders(w, decision)

			if !decision.Allowed {
				onLimit(w, req, decision)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func (r *RateLimiter) Decide(req *http.Request) (Decision, bool) {
	if req == nil {
		return Decision{}, true
	}

	path := req.URL.Path
	if _, ok := r.exemptPaths[path]; ok {
		return Decision{}, true
	}

	now := r.now()
	policy := r.policyForPath(path)
	clientKey := clientIP(req, r.trustProxy)
	key := policy.Name + "|" + clientKey

	r.mu.Lock()
	defer r.mu.Unlock()

	r.maybeCleanupLocked(now)

	entry, ok := r.buckets[key]
	if !ok {
		entry = &bucketEntry{
			Tokens:     float64(policy.Requests),
			LastRefill: now,
		}
		r.buckets[key] = entry
	}

	refillBucket(entry, now, policy)

	decision := Decision{
		Allowed:    true,
		Limit:      policy.Requests,
		ClientKey:  clientKey,
		PolicyName: policy.Name,
	}

	if entry.Tokens < 1 {
		retryAfter := timeToNextToken(entry, policy)
		decision.Allowed = false
		decision.RetryAfter = retryAfter
		decision.ResetAfter = retryAfter
		decision.Remaining = 0
		return decision, false
	}

	entry.Tokens--
	decision.Remaining = int(math.Floor(entry.Tokens))
	decision.ResetAfter = timeToFullBucket(entry, policy)
	return decision, false
}

func (r *RateLimiter) policyForPath(path string) Policy {
	if policy, ok := r.routePolicies[path]; ok {
		return policy
	}
	return r.defaultPolicy
}

func (r *RateLimiter) maybeCleanupLocked(now time.Time) {
	if r.cleanupInterval <= 0 {
		return
	}
	if !r.lastCleanup.IsZero() && now.Sub(r.lastCleanup) < r.cleanupInterval {
		return
	}

	maxWindow := r.defaultPolicy.Window
	for _, policy := range r.routePolicies {
		if policy.Window > maxWindow {
			maxWindow = policy.Window
		}
	}

	for key, bucket := range r.buckets {
		if now.Sub(bucket.LastRefill) >= maxWindow {
			delete(r.buckets, key)
		}
	}
	r.lastCleanup = now
}

func refillBucket(entry *bucketEntry, now time.Time, policy Policy) {
	if !now.After(entry.LastRefill) {
		return
	}

	ratePerSecond := float64(policy.Requests) / policy.Window.Seconds()
	elapsed := now.Sub(entry.LastRefill).Seconds()
	entry.Tokens = math.Min(float64(policy.Requests), entry.Tokens+(elapsed*ratePerSecond))
	entry.LastRefill = now
}

func timeToNextToken(entry *bucketEntry, policy Policy) time.Duration {
	if entry.Tokens >= 1 {
		return 0
	}
	ratePerSecond := float64(policy.Requests) / policy.Window.Seconds()
	missing := 1 - entry.Tokens
	seconds := missing / ratePerSecond
	return time.Duration(math.Ceil(seconds * float64(time.Second)))
}

func timeToFullBucket(entry *bucketEntry, policy Policy) time.Duration {
	if entry.Tokens >= float64(policy.Requests) {
		return 0
	}
	ratePerSecond := float64(policy.Requests) / policy.Window.Seconds()
	missing := float64(policy.Requests) - entry.Tokens
	seconds := missing / ratePerSecond
	return time.Duration(math.Ceil(seconds * float64(time.Second)))
}

func setRateLimitHeaders(w http.ResponseWriter, decision Decision) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(decision.Remaining, 0)))
	if decision.ResetAfter > 0 {
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(math.Ceil(decision.ResetAfter.Seconds()))))
	}
	if !decision.Allowed && decision.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(decision.RetryAfter.Seconds()))))
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
			parts := strings.Split(forwardedFor, ",")
			if len(parts) > 0 {
				ip := strings.TrimSpace(parts[0])
				if ip != "" {
					return ip
				}
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
