package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	exporter := &exporter{}
	request := httptest.NewRequest(http.MethodGet, "/health-check", nil)

	recorder := httptest.NewRecorder()
	exporter.healthHandler(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("disconnected status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	exporter.connected.Store(true)
	recorder = httptest.NewRecorder()
	exporter.healthHandler(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("connected but not subscribed status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	exporter.subscribed.Store(true)
	recorder = httptest.NewRecorder()
	exporter.healthHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("healthy status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != "ok\n" {
		t.Fatalf("healthy body = %q, want %q", body, "ok\n")
	}
}
