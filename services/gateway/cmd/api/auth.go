package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/peer-ledger/internal/security"
)

type authContextKey string

const userClaimsContextKey authContextKey = "userClaims"

func (app *Config) authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := app.authenticateRequest(r)
			if err != nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				_ = app.errorJSON(w, err, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userClaimsContextKey, claims)))
		})
	}
}

func (app *Config) authenticateRequest(r *http.Request) (*security.JWTClaims, error) {
	if app.tokenManager == nil {
		return nil, errors.New("authentication is not configured")
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return nil, errors.New("authorization token is required")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return nil, errors.New("authorization header must be Bearer <token>")
	}

	claims, err := app.tokenManager.Parse(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, errors.New("invalid or expired token")
	}

	return claims, nil
}

func claimsFromContext(ctx context.Context) (*security.JWTClaims, bool) {
	claims, ok := ctx.Value(userClaimsContextKey).(*security.JWTClaims)
	return claims, ok
}
