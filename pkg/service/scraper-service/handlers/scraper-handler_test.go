package handlers

import (
	"testing"

	"project-phoenix/v2/internal/model"
)

func TestNewProviderKeyPatterns(t *testing.T) {
	tests := map[string]string{
		model.ProviderMiniMax: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.signature_value",
		model.ProviderTencent: "sk-tp-1234567890abcdefghijklmnop",
		model.ProviderStepFun: "sk-1234567890abcdefghijklmnop",
		model.ProviderQwen:    "sk-sp-1234567890abcdefghijklmnop",
		model.ProviderMistral: "1234567890abcdefghijklmnopqrstuv",
	}

	for provider, key := range tests {
		matches := KeyPatterns[provider].FindAllString("API_KEY="+key, -1)
		if len(matches) != 1 || matches[0] != key {
			t.Fatalf("%s pattern returned %q, want %q", provider, matches, key)
		}
	}
}
