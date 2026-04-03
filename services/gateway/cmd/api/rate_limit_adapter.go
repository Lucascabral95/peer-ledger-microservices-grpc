package main

import (
	"net/http"

	gatewaymiddleware "github.com/peer-ledger/services/gateway/internal/middleware"
)

func (app *Config) rateLimitMiddleware() func(http.Handler) http.Handler {
	if app.rateLimiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return app.rateLimiter.Middleware(func(w http.ResponseWriter, r *http.Request, decision gatewaymiddleware.Decision) {
		_ = app.writeJSON(w, http.StatusTooManyRequests, jsonResponse{
			Error:   true,
			Message: "rate limit exceeded",
			Data: map[string]any{
				"client_ip":   decision.ClientKey,
				"policy":      decision.PolicyName,
				"retry_after": decision.RetryAfter.String(),
			},
		})
	})
}
