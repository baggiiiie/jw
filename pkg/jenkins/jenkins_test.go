package jenkins

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetJobStatus(t *testing.T) {
	tests := []struct {
		name         string
		expected     JobStatus
		expectStatus int
		wantErr      bool
		setupAuth    bool
	}{
		{
			name: "success with job building",
			expected: JobStatus{
				Building:  true,
				Result:    "",
				Timestamp: 1234567890,
			},
			expectStatus: http.StatusOK,
			wantErr:      false,
			setupAuth:    true,
		},
		{
			name: "success with job finished",
			expected: JobStatus{
				Building:  false,
				Result:    "SUCCESS",
				Timestamp: 1234567890,
			},
			expectStatus: http.StatusOK,
			wantErr:      false,
			setupAuth:    false,
		},
		{
			name: "failure result",
			expected: JobStatus{
				Building:  false,
				Result:    "FAILURE",
				Timestamp: 1234567890,
			},
			expectStatus: http.StatusOK,
			wantErr:      false,
			setupAuth:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/json", r.URL.Path, "expected path /api/json")
				capturedAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tt.expected)
			}))
			defer server.Close()

			token := "test-token"
			if !tt.setupAuth {
				token = ""
			}

			status, code, err := GetJobStatus(server.URL, token)
			assert.Equal(t, tt.wantErr, err != nil, "unexpected error")
			assert.Equal(t, tt.expectStatus, code, "expected status %d, got %d", tt.expectStatus, code)
			assert.Equal(t, tt.expected.Building, status.Building, "Building mismatch")
			assert.Equal(t, tt.expected.Result, status.Result, "Result mismatch")
			assert.Equal(t, tt.expected.Timestamp, status.Timestamp, "Timestamp mismatch")
			if tt.setupAuth {
				assert.Equal(t, "Basic test-token", capturedAuth, "Authorization header mismatch")
			}
		})
	}
}
