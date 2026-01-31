package jenkins

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetJobStatus_Success(t *testing.T) {
	expected := JobStatus{
		Building:  true,
		Result:    "",
		Timestamp: 1234567890,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/api/json" {
			t.Errorf("expected path /api/json, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Basic test-token" {
			t.Errorf("expected Authorization header 'Basic test-token', got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	status, code, err := GetJobStatus(server.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("expected status 200, got %d", code)
	}
	if status.Building != expected.Building {
		t.Errorf("expected Building=%v, got %v", expected.Building, status.Building)
	}
	if status.Timestamp != expected.Timestamp {
		t.Errorf("expected Timestamp=%d, got %d", expected.Timestamp, status.Timestamp)
	}
}

func TestGetJobStatus_SuccessFinished(t *testing.T) {
	expected := JobStatus{
		Building:  false,
		Result:    "SUCCESS",
		Timestamp: 1234567890,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	status, code, err := GetJobStatus(server.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("expected status 200, got %d", code)
	}
	if status.Building != false {
		t.Error("expected Building=false")
	}
	if status.Result != "SUCCESS" {
		t.Errorf("expected Result=SUCCESS, got %s", status.Result)
	}
}

func TestGetJobStatus_FailureResult(t *testing.T) {
	expected := JobStatus{
		Building:  false,
		Result:    "FAILURE",
		Timestamp: 1234567890,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	status, _, err := GetJobStatus(server.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Result != "FAILURE" {
		t.Errorf("expected Result=FAILURE, got %s", status.Result)
	}
}
