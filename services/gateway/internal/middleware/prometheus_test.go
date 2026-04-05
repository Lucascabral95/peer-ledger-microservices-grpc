package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
)

func TestHTTPMetricsMiddleware_RecordsRouteAndStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)

	router := chi.NewRouter()
	router.Use(metrics.Middleware())
	router.Get("/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/user-001", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	var found bool
	for _, family := range families {
		if family.GetName() != "gateway_http_requests_total" {
			continue
		}

		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			if labels["method"] == http.MethodGet && labels["route"] == "/users/{userID}" && labels["status"] == "201" {
				found = true
			}
		}
	}

	if !found {
		t.Fatalf("expected prometheus metric with method, route pattern and status labels")
	}
}
