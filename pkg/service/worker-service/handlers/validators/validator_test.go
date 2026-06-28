package validators

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

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
