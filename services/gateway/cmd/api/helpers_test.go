package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSON_Success(t *testing.T) {
	app := &Config{}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"sender_id":"user-001"}`))
	rr := httptest.NewRecorder()

	var payload map[string]string
	if err := app.readJSON(rr, req, &payload); err != nil {
		t.Fatalf("readJSON() unexpected error: %v", err)
	}
	if payload["sender_id"] != "user-001" {
		t.Fatalf("expected sender_id user-001, got %q", payload["sender_id"])
	}
}

func TestReadJSON_MultipleValues(t *testing.T) {
	app := &Config{}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1} {"b":2}`))
	rr := httptest.NewRecorder()

	var payload map[string]any
	err := app.readJSON(rr, req, &payload)
	if err == nil {
		t.Fatalf("expected error for multiple JSON values")
	}
	if err.Error() != "body must have only a single JSON value" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteJSON_SetsHeadersAndBody(t *testing.T) {
	app := &Config{}
	rr := httptest.NewRecorder()
	headers := http.Header{}
	headers.Set("X-Test", "ok")

	err := app.writeJSON(rr, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: "created",
	}, headers)
	if err != nil {
		t.Fatalf("writeJSON() unexpected error: %v", err)
	}

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %q", rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("X-Test") != "ok" {
		t.Fatalf("expected X-Test header")
	}
	if !strings.Contains(rr.Body.String(), `"message":"created"`) {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
}

func TestErrorJSON_DefaultStatus(t *testing.T) {
	app := &Config{}
	rr := httptest.NewRecorder()

	err := app.errorJSON(rr, errors.New("bad request"))
	if err != nil {
		t.Fatalf("errorJSON() unexpected error: %v", err)
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"message":"bad request"`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}
