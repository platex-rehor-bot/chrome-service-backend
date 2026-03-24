package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetOutputPath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "uses default when env not set",
			envValue: "",
			want:     defaultOutputPath,
		},
		{
			name:     "uses env value when set",
			envValue: "custom/path/specs.json",
			want:     "custom/path/specs.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(envOutputPath, tt.envValue)
				defer os.Unsetenv(envOutputPath)
			} else {
				os.Unsetenv(envOutputPath)
			}

			got := getOutputPath()
			if got != tt.want {
				t.Errorf("getOutputPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHTTPTimeout(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     time.Duration
	}{
		{
			name:     "uses default when env not set",
			envValue: "",
			want:     defaultTimeout,
		},
		{
			name:     "uses env value when set to valid number",
			envValue: "30",
			want:     30 * time.Second,
		},
		{
			name:     "uses default when env value is invalid",
			envValue: "invalid",
			want:     defaultTimeout,
		},
		{
			name:     "uses default when env value is negative",
			envValue: "-10",
			want:     defaultTimeout,
		},
		{
			name:     "uses default when env value is zero",
			envValue: "0",
			want:     defaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(envHTTPTimeout, tt.envValue)
				defer os.Unsetenv(envHTTPTimeout)
			} else {
				os.Unsetenv(envHTTPTimeout)
			}

			got := getHTTPTimeout()
			if got != tt.want {
				t.Errorf("getHTTPTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchURL(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		wantStatus     int
		wantErr        bool
		wantBodyEmpty  bool
	}{
		{
			name:           "successful fetch with valid JSON",
			responseBody:   `{"openapi":"3.0.0"}`,
			responseStatus: http.StatusOK,
			wantStatus:     http.StatusOK,
			wantErr:        false,
			wantBodyEmpty:  false,
		},
		{
			name:           "handles 404 response",
			responseBody:   "not found",
			responseStatus: http.StatusNotFound,
			wantStatus:     http.StatusNotFound,
			wantErr:        false,
			wantBodyEmpty:  false,
		},
		{
			name:           "handles 500 response",
			responseBody:   "internal error",
			responseStatus: http.StatusInternalServerError,
			wantStatus:     http.StatusInternalServerError,
			wantErr:        false,
			wantBodyEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept") != "application/json" {
					t.Errorf("Expected Accept header to be 'application/json', got '%s'", r.Header.Get("Accept"))
				}
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			body, status, err := fetchURL(client, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if status != tt.wantStatus {
				t.Errorf("fetchURL() status = %v, want %v", status, tt.wantStatus)
			}

			if tt.wantBodyEmpty && len(body) > 0 {
				t.Errorf("fetchURL() body should be empty, got %d bytes", len(body))
			}

			if !tt.wantBodyEmpty && len(body) == 0 {
				t.Errorf("fetchURL() body should not be empty")
			}
		})
	}
}

func TestFetchURLTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	_, _, err := fetchURL(client, server.URL)

	if err == nil {
		t.Error("fetchURL() should timeout but didn't return error")
	}
}

func TestWriteOutput(t *testing.T) {
	tempDir := t.TempDir()

	// Change to temp directory so writeOutput's filepath.Abs(".") works correctly
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	outputPath := "test-specs.json"

	testData := map[string][]specOutputEntry{
		"test-service": {
			{
				URL:          "https://example.com/api/v1/openapi.json",
				BundleLabels: []string{"test"},
				Spec:         json.RawMessage(`{"openapi":"3.0.0"}`),
			},
		},
	}

	err = writeOutput(outputPath, testData)
	if err != nil {
		t.Fatalf("writeOutput() error = %v", err)
	}

	fullPath := filepath.Join(tempDir, outputPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("writeOutput() did not create file at %s", fullPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var result map[string][]specOutputEntry
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output file contains invalid JSON: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 service, got %d", len(result))
	}

	if _, ok := result["test-service"]; !ok {
		t.Error("expected 'test-service' key in result")
	}
}

func TestWriteOutputCreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Change to temp directory so writeOutput's filepath.Abs(".") works correctly
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	outputPath := "nested/dir/specs.json"

	testData := map[string][]specOutputEntry{}

	err = writeOutput(outputPath, testData)
	if err != nil {
		t.Fatalf("writeOutput() error = %v", err)
	}

	fullPath := filepath.Join(tempDir, outputPath)
	if _, err := os.Stat(filepath.Dir(fullPath)); os.IsNotExist(err) {
		t.Error("writeOutput() did not create parent directories")
	}
}

func TestAPISpecInputValidation(t *testing.T) {
	tests := []struct {
		name  string
		entry apiSpecInput
		valid bool
	}{
		{
			name: "valid entry",
			entry: apiSpecInput{
				URL:          "https://example.com/api.json",
				FrontendName: "test-service",
				BundleLabels: []string{"test"},
			},
			valid: true,
		},
		{
			name: "empty URL",
			entry: apiSpecInput{
				URL:          "",
				FrontendName: "test-service",
			},
			valid: false,
		},
		{
			name: "empty frontendName",
			entry: apiSpecInput{
				URL:          "https://example.com/api.json",
				FrontendName: "",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.entry.URL != "" && tt.entry.FrontendName != ""
			if isValid != tt.valid {
				t.Errorf("validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}
