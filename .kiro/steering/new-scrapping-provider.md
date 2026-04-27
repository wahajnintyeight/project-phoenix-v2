---
inclusion: manual
---

# Adding a New Scraping Provider

This guide provides step-by-step instructions for adding a new API provider to the scraping and validation system.

## Overview

The system has two main components:
1. **Scraper Service**: Searches GitHub/GitLab for exposed API keys using search patterns
2. **Worker Service**: Validates discovered keys against provider APIs

## Step-by-Step Implementation

### 1. Add Provider Constant

**File**: `internal/model/api-key.go`

Add your provider to the constants section:

```go
const (
    ProviderOpenAI     = "OpenAI"
    ProviderAnthropic  = "Anthropic"
    ProviderGoogle     = "Google"
    ProviderOpenRouter = "OpenRouter"
    ProviderMoonshot   = "Moonshot"
    ProviderYourNew    = "YourNew"  // Add here - use PascalCase
)
```

### 2. Add Search Patterns

**File**: `internal/service-configs/scraper-service/search-patterns.json`

Add a new provider entry with search queries:

```json
{
  "provider": "YourNew",
  "enabled": true,
  "queries": [
    {
      "pattern": "\"your-key-prefix-\" ",
      "description": "YourNew API keys in .env files"
    },
    {
      "pattern": "\"YOURNEW_API_KEY\" ",
      "description": "YourNew environment variable"
    }
  ]
}
```

**Search Pattern Tips**:
- Use quotes for exact matches
- Include spaces after patterns to avoid partial matches
- Target common variable names and key prefixes
- Test patterns on GitHub search first: https://github.com/search?type=code

### 3. Add Key Extraction Pattern (Optional)

**File**: `pkg/service/scraper-service/handlers/scraper-handler.go`

Add regex pattern to extract keys from file content:

```go
var KeyPatterns = map[string]*regexp.Regexp{
    model.ProviderOpenAI:     regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`),
    model.ProviderAnthropic:  regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-]{95,}`),
    model.ProviderYourNew:    regexp.MustCompile(`yn-[a-zA-Z0-9]{32,64}`),  // Add here
}
```

**Note**: This is optional. If omitted, the scraper will store the entire matched line.

### 4. Create Validator

**File**: `pkg/service/worker-service/handlers/validators/yournew.go`

Create a new validator file:

```go
package validators

import (
    "bytes"
    "encoding/json"
    "net/http"
    
    "project-phoenix/v2/internal/model"
)

// YourNewValidator validates YourNew API keys
type YourNewValidator struct {
    *BaseValidator
}

// NewYourNewValidator creates a new YourNew validator
func NewYourNewValidator(debugMode bool) *YourNewValidator {
    return &YourNewValidator{
        BaseValidator: NewBaseValidator(debugMode),
    }
}

// GetProviderName returns the provider name
func (v *YourNewValidator) GetProviderName() string {
    return model.ProviderYourNew
}

// Validate validates a YourNew API key
func (v *YourNewValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
    // API endpoint to test the key
    url := "https://api.yournew.com/v1/chat/completions"
    
    // Minimal request body to test authentication
    requestBody := map[string]interface{}{
        "model": "your-cheapest-model",
        "max_tokens": 1,
        "messages": []map[string]string{
            {"role": "user", "content": "hi"},
        },
    }
    
    bodyBytes, err := json.Marshal(requestBody)
    if err != nil {
        return model.StatusError, nil, err
    }
    
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
    if err != nil {
        return model.StatusError, nil, err
    }
    
    // Set authentication header (varies by provider)
    req.Header.Set("Authorization", "Bearer "+keyValue)
    // OR: req.Header.Set("X-API-Key", keyValue)
    req.Header.Set("Content-Type", "application/json")
    
    // ExecuteRequestWithRetry handles retries and status determination
    status, err := v.ExecuteRequestWithRetry(req, correlationID)
    return status, nil, err
}
```

**Validator Best Practices**:
- Use the cheapest/fastest endpoint available
- Request minimal tokens (1-10) to reduce costs
- Use BaseValidator's `ExecuteRequestWithRetry` for automatic retry logic
- Return `model.StatusError` for temporary failures (5xx)
- Return `model.StatusInvalid` for auth failures (401, 403)

### 5. Register Validator in Factory

**File**: `pkg/service/worker-service/handlers/validators/factory.go`

Add your validator to the registration function:

```go
func (f *ValidatorFactory) registerValidators() {
    f.Register(NewOpenAIValidator(f.debugMode))
    f.Register(NewAnthropicValidator(f.debugMode))
    f.Register(NewGoogleValidator(f.debugMode))
    f.Register(NewOpenRouterValidator(f.debugMode))
    f.Register(NewMoonshotValidator(f.debugMode))
    f.Register(NewYourNewValidator(f.debugMode))  // Add here
}
```

## Advanced: Credits Support

If the provider has a credits/balance API, you can return credit information:

```go
func (v *YourNewValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
    // First validate the key
    status, err := v.ExecuteRequestWithRetry(req, correlationID)
    if err != nil || status != model.StatusValid {
        return status, nil, err
    }
    
    // Then fetch credits (if provider has a credits endpoint)
    creditsURL := "https://api.yournew.com/v1/credits"
    creditsReq, _ := http.NewRequest("GET", creditsURL, nil)
    creditsReq.Header.Set("Authorization", "Bearer "+keyValue)
    
    resp, err := v.HTTPClient.Do(creditsReq)
    if err != nil {
        return model.StatusValid, nil, nil // Key is valid, just no credits info
    }
    defer resp.Body.Close()
    
    var creditsData map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&creditsData)
    
    credits := map[string]interface{}{
        "balance":    creditsData["balance"],
        "checked_at": time.Now(),
    }
    
    // Determine if key has credits
    if balance, ok := creditsData["balance"].(float64); ok && balance <= 0 {
        return model.StatusValidNoCredits, credits, nil
    }
    
    return model.StatusValid, credits, nil
}
```

## Status Codes Reference

Your validator should return these statuses:

- **`StatusValid`**: Key is valid and has credits/quota
- **`StatusValidNoCredits`**: Key is valid but has no credits remaining
- **`StatusInvalid`**: Key is invalid (401, 403, 429, 400 responses)
- **`StatusError`**: Temporary error or unknown issue (5xx, network errors)

## HTTP Status Code Mapping

The `BaseValidator.DetermineStatusFromResponse()` maps HTTP codes:

- `200` → `StatusValid`
- `401, 403, 429, 400` → `StatusInvalid`
- `5xx` → `StatusError` (with automatic retry)


## File Checklist

When adding a new provider, you'll modify these files:

- [ ] `internal/model/api-key.go` - Add provider constant
- [ ] `internal/service-configs/scraper-service/search-patterns.json` - Add search patterns
- [ ] `pkg/service/scraper-service/handlers/scraper-handler.go` - Add key extraction regex (optional)
- [ ] `pkg/service/worker-service/handlers/validators/yournew.go` - Create validator (new file)
- [ ] `pkg/service/worker-service/handlers/validators/factory.go` - Register validator

## Examples

See existing implementations:
- **Simple validator**: `validators/openai.go`
- **Custom headers**: `validators/anthropic.go` (uses `X-Api-Key` instead of `Authorization`)
- **Credits support**: `validators/openrouter.go`

## Common Pitfalls

1. **Forgetting to register validator** - Always add to `factory.go`
2. **Wrong header format** - Check provider docs for auth header format
3. **Expensive API calls** - Use cheapest endpoint with minimal tokens
4. **Case sensitivity** - Provider constants use PascalCase
5. **Search pattern too broad** - Test on GitHub first to avoid false positives

## Need Help?

- Check existing validators in `pkg/service/worker-service/handlers/validators/`
- Review `validators/README.md` for detailed architecture
- Enable `VERIFIER_DEBUG=true` to see API responses
- Test search patterns on GitHub before adding them 