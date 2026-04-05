package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
}

func NewHTTPMetrics(registerer prometheus.Registerer) *HTTPMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	m := &HTTPMetrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_http_requests_total",
				Help: "Total HTTP requests processed by the gateway.",
			},
			[]string{"method", "route", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds for the gateway.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
	}

	registerer.MustRegister(m.requestsTotal, m.requestDuration)
	return m
}

func (m *HTTPMetrics) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusCapturingResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(ww, r)

			route := routePattern(r)
			statusCode := strconv.Itoa(ww.status)

			m.requestsTotal.WithLabelValues(r.Method, route, statusCode).Inc()
			m.requestDuration.WithLabelValues(r.Method, route, statusCode).Observe(time.Since(start).Seconds())
		})
	}
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func routePattern(r *http.Request) string {
	if r == nil {
		return "unknown"
	}

	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := strings.TrimSpace(rctx.RoutePattern()); pattern != "" {
			return pattern
		}
	}

	if r.URL != nil && strings.TrimSpace(r.URL.Path) != "" {
		return strings.TrimSpace(r.URL.Path)
	}

	return "unknown"
}
