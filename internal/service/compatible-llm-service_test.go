package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"project-phoenix/v2/internal/model"
)

type llmRoundTripFunc func(*http.Request) (*http.Response, error)

func (f llmRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSendChatCompletionOpenAICompatibleProviders(t *testing.T) {
	providers := []string{"huggingface", "xai", "mistral", "deepseek", "gemini"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			client := &http.Client{Transport: llmRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != openAICompatibleLLMProviders[provider] {
					t.Errorf("request URL = %q, want %q", req.URL.String(), openAICompatibleLLMProviders[provider])
				}
				if got := req.Header.Get("Authorization"); got != "Bearer secret" {
					t.Errorf("Authorization = %q, want bearer key", got)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("read request body: %v", err)
				}
				bodyText := string(body)
				for _, expected := range []string{`"model":"test-model"`, `"role":"user"`, `"content":"hello"`, `"max_tokens":64`} {
					if !strings.Contains(bodyText, expected) {
						t.Errorf("request body %s missing %s", bodyText, expected)
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id":"response-1","model":"test-model","choices":[{"message":{"content":"hello back"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)),
				}, nil
			})}

			response, err := (&LLMService{client: client}).SendChatCompletion(model.ChatCompletionRequest{
				Provider:  provider,
				APIKey:    "secret",
				Model:     "test-model",
				MaxTokens: 64,
				Messages:  []model.ChatMessage{{Role: "user", Content: "hello"}},
			})
			if err != nil {
				t.Fatalf("SendChatCompletion() error = %v", err)
			}
			if response.Message.Content != "hello back" || response.Usage.TotalTokens != 5 {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

func TestIsSupportedLLMProvider(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic", "groq", "openrouter", "ollama", "huggingface", "xai", "mistral", "deepseek", "gemini", " XAI "} {
		if !IsSupportedLLMProvider(provider) {
			t.Errorf("IsSupportedLLMProvider(%q) = false", provider)
		}
	}
	if IsSupportedLLMProvider("unknown") {
		t.Error("IsSupportedLLMProvider(unknown) = true")
	}
}
