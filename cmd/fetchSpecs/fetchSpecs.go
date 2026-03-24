package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultOutputPath = "static/specs-generated.json"
	envFEOAPISpec     = "FEO_API_SPEC"
	envOutputPath     = "FETCH_SPECS_TARGET"
	envHTTPTimeout    = "FETCH_SPECS_TIMEOUT"
	defaultTimeout    = 60 * time.Second
	maxBodySize       = 32 << 20 // 32MB
)

// apiSpecInput matches FEO_API_SPEC / frontend-operator APISpecInfo (see frontend_types.go).
type apiSpecInput struct {
	URL          string   `json:"url"`
	BundleLabels []string `json:"bundleLabels"`
	FrontendName string   `json:"frontendName"`
}

// specOutputEntry is one resolved OpenAPI document plus metadata from the index.
type specOutputEntry struct {
	URL          string          `json:"url"`
	BundleLabels []string        `json:"bundleLabels"`
	Spec         json.RawMessage `json:"spec"`
}

func main() {
	_ = godotenv.Load()

	outputPath := getOutputPath()
	timeout := getHTTPTimeout()

	raw := os.Getenv(envFEOAPISpec)
	if raw == "" {
		fmt.Fprintf(os.Stderr, "%s is not set; writing empty object to %s\n", envFEOAPISpec, outputPath)
		if err := writeOutput(outputPath, map[string][]specOutputEntry{}); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var entries []apiSpecInput
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s JSON: %v\n", envFEOAPISpec, err)
		os.Exit(1)
	}

	out := make(map[string][]specOutputEntry)
	client := &http.Client{Timeout: timeout}

	for _, e := range entries {
		if e.URL == "" {
			fmt.Fprintf(os.Stderr, "skipping entry with empty url (frontendName=%q)\n", e.FrontendName)
			continue
		}
		if e.FrontendName == "" {
			fmt.Fprintf(os.Stderr, "skipping entry with empty frontendName (url=%q)\n", e.URL)
			continue
		}

		body, status, err := fetchURL(client, e.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch failed for %q: %v\n", e.URL, err)
			continue
		}
		if status < 200 || status >= 300 {
			fmt.Fprintf(os.Stderr, "fetch failed for %q: HTTP %d\n", e.URL, status)
			continue
		}

		if !json.Valid(body) {
			fmt.Fprintf(os.Stderr, "response is not valid JSON for %q; skipping\n", e.URL)
			continue
		}

		out[e.FrontendName] = append(out[e.FrontendName], specOutputEntry{
			URL:          e.URL,
			BundleLabels: e.BundleLabels,
			Spec:         json.RawMessage(body),
		})
	}

	if err := writeOutput(outputPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s (%d frontend keys)\n", outputPath, len(out))
}

func getOutputPath() string {
	if path := os.Getenv(envOutputPath); path != "" {
		return path
	}
	return defaultOutputPath
}

func getHTTPTimeout() time.Duration {
	if timeoutStr := os.Getenv(envHTTPTimeout); timeoutStr != "" {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultTimeout
}

func fetchURL(client *http.Client, url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodySize))
	if err != nil {
		return nil, res.StatusCode, err
	}
	return body, res.StatusCode, nil
}

func writeOutput(outputPath string, data map[string][]specOutputEntry) error {
	cwd, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	path := filepath.Join(cwd, outputPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}
