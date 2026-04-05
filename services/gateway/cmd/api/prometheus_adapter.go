package main

import "net/http"

func (app *Config) prometheusMiddleware() func(http.Handler) http.Handler {
	if app.httpMetrics == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return app.httpMetrics.Middleware()
}
