package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type keyTesterRoundTripFunc func(*http.Request) (*http.Response, error)

func (f keyTesterRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTestXAIKeyUsesLatestGrokModel(t *testing.T) {
	originalClient := keyTesterHTTPClient
	t.Cleanup(func() { keyTesterHTTPClient = originalClient })

	keyTesterHTTPClient = &http.Client{
		Transport: keyTesterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %q, want POST", req.Method)
			}
			if req.URL.String() != "https://api.x.ai/v1/chat/completions" {
				t.Fatalf("URL = %q, want xAI chat completions endpoint", req.URL.String())
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if got := payload["model"]; got != "grok-4.7" {
				t.Fatalf("model = %v, want grok-4.7", got)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"id":"test"}`)),
			}, nil
		}),
	}

	result := testXAIKey("  test-key  ", "")
	if result.Status != "Valid" {
		t.Fatalf("status = %q, want Valid (error: %v)", result.Status, result.Err)
	}
}

func TestTestXAIKeyHonorsModelOverride(t *testing.T) {
	originalClient := keyTesterHTTPClient
	t.Cleanup(func() { keyTesterHTTPClient = originalClient })

	keyTesterHTTPClient = &http.Client{
		Transport: keyTesterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if got := payload["model"]; got != "grok-custom" {
				t.Fatalf("model = %v, want grok-custom", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"id":"test"}`)),
			}, nil
		}),
	}

	result := testXAIKey("test-key", "grok-custom")
	if result.Status != "Valid" {
		t.Fatalf("status = %q, want Valid (error: %v)", result.Status, result.Err)
	}
}
