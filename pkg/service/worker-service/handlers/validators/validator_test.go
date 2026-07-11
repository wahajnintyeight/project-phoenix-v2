package validators

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"project-phoenix/v2/internal/model"
)

func TestExecuteRequestWithRetryResetsRequestBody(t *testing.T) {
	var calls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()

		if string(body) != `{"ping":true}` {
			t.Fatalf("body = %q, want request payload", body)
		}

		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("POST", server.URL, strings.NewReader(`{"ping":true}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err := NewBaseValidator(false).ExecuteRequestWithRetry(req, "test")
	if err != nil {
		t.Fatal(err)
	}

	if status != model.StatusValid {
		t.Fatalf("status = %q, want %q", status, model.StatusValid)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestExecuteRequestWithRetryRetriesTransportTimeout(t *testing.T) {
	var calls int32
	validator := NewBaseValidator(false)
	validator.HTTPClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, &url.Error{
				Op:  "Post",
				URL: "https://generativelanguage.googleapis.com/v1beta/models/test:generateContent?key=secret",
				Err: context.DeadlineExceeded,
			}
		}

		return nil, &url.Error{
			Op:  "Post",
			URL: "https://generativelanguage.googleapis.com/v1beta/models/test:generateContent?key=secret",
			Err: context.DeadlineExceeded,
		}
	})

	req, err := http.NewRequest("POST", "https://example.com", strings.NewReader(`{"ping":true}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err := validator.ExecuteRequestWithRetry(req, "test")
	if err == nil {
		t.Fatal("expected error")
	}
	if status != model.StatusError {
		t.Fatalf("status = %q, want %q", status, model.StatusError)
	}
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("calls = %d, want 4", got)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked key: %v", err)
	}
	if !strings.Contains(err.Error(), "%5BREDACTED%5D") {
		t.Fatalf("error = %q, want redacted key", err)
	}
}

func TestZAIValidatorUsesLongerTimeout(t *testing.T) {
	if got := NewZAIValidator(false).HTTPClient.Timeout; got != 50*time.Second {
		t.Fatalf("timeout = %v, want 50s", got)
	}
}

func TestFactoryRegistersNewCodingProviders(t *testing.T) {
	factory := NewValidatorFactory(false)
	for _, provider := range []string{
		model.ProviderMiniMax,
		model.ProviderTencent,
		model.ProviderStepFun,
		model.ProviderQwen,
		model.ProviderMistral,
	} {
		if _, err := factory.GetValidator(provider); err != nil {
			t.Fatalf("provider %s is not registered: %v", provider, err)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
