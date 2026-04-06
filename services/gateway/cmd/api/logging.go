package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
)

type logContextKey string

const (
	requestIDContextKey logContextKey = "requestID"
	traceIDContextKey   logContextKey = "traceID"
)

type accessLogResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *accessLogResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *accessLogResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (app *Config) requestContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if requestID == "" {
				requestID = newRequestID()
			}

			traceID := strings.TrimSpace(r.Header.Get("X-Trace-ID"))
			if traceID == "" {
				traceID = requestID
			}

			w.Header().Set("X-Request-ID", requestID)

			ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
			ctx = context.WithValue(ctx, traceIDContextKey, traceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (app *Config) httpAccessLogMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			ww := &accessLogResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(ww, r)

			route := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := strings.TrimSpace(rctx.RoutePattern()); pattern != "" {
					route = pattern
				}
			}

			userID := strings.TrimSpace(ww.Header().Get("X-User-ID"))
			if userID == "" {
				userID = "anonymous"
			}

			app.logEvent(r.Context(), "info", "http request completed", map[string]any{
				"method":      r.Method,
				"route":       route,
				"status":      ww.status,
				"latency_ms":  time.Since(startedAt).Milliseconds(),
				"bytes":       ww.bytes,
				"remote_addr": remoteIP(r.RemoteAddr),
				"user_id":     userID,
			})
		})
	}
}

func (app *Config) logEvent(ctx context.Context, level, message string, fields map[string]any) {
	payload := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"level":   strings.ToLower(strings.TrimSpace(level)),
		"service": "gateway",
		"message": message,
	}

	if requestID := requestIDFromContext(ctx); requestID != "" {
		payload["request_id"] = requestID
	}
	if traceID := traceIDFromContext(ctx); traceID != "" {
		payload["trace_id"] = traceID
	}

	for k, v := range fields {
		payload[k] = v
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}

	encoded = append(encoded, '\n')
	_, _ = os.Stdout.Write(encoded)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(requestIDContextKey).(string); ok {
		return strings.TrimSpace(requestID)
	}
	return ""
}

func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(traceIDContextKey).(string); ok {
		return strings.TrimSpace(traceID)
	}
	return ""
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}
