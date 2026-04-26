# Provider Validators

This package contains the factory pattern implementation for API key validation across different providers.

## Architecture

The validator system uses a factory pattern to make it easy to add new providers without modifying existing code:

- **Validator Interface**: Defines the contract all validators must implement
- **BaseValidator**: Provides common functionality (HTTP client, retry logic, status determination)
- **Provider-Specific Validators**: Individual files for each provider (OpenAI, Anthropic, Google, OpenRouter, Moonshot)
- **ValidatorFactory**: Manages validator registration and retrieval

## Adding a New Provider

To add support for a new API provider, follow these steps:

### 1. Add Provider Constant

Add the provider constant to `internal/model/api-key.go`:

```go
const (
    ProviderOpenAI     = "OpenAI"
    ProviderAnthropic  = "Anthropic"
    ProviderGoogle     = "Google"
    ProviderOpenRouter = "OpenRouter"
    ProviderMoonshot   = "Moonshot"
    ProviderYourNew    = "YourNew"  // Add your provider here
)
```

### 2. Create Validator File

Create a new file `validators/yournew.go`:

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
    url := "https://api.yournew.com/v1/endpoint"
    
    requestBody := map[string]interface{}{
        "model": "your-model",
        "messages": []map[string]string{
            {"role": "user", "content": "test"},
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
    
    req.Header.Set("Authorization", "Bearer "+keyValue)
    req.Header.Set("Content-Type", "application/json")
    
    status, err := v.ExecuteRequestWithRetry(req, correlationID)
    return status, nil, err
}
```

### 3. Register Validator

Add your validator to the factory in `validators/factory.go`:

```go
func (f *ValidatorFactory) registerValidators() {
    f.Register(NewOpenAIValidator(f.debugMode))
    f.Register(NewAnthropicValidator(f.debugMode))
    f.Register(NewGoogleValidator(f.debugMode))
    f.Register(NewOpenRouterValidator(f.debugMode))
    f.Register(NewMoonshotValidator(f.debugMode))
    f.Register(NewYourNewValidator(f.debugMode))  // Add your validator here
}
```

### 4. Add Key Pattern (Optional)

If you want to scrape keys for this provider, add a regex pattern in `pkg/service/scraper-service/handlers/scraper-handler.go`:

```go
var KeyPatterns = map[string]*regexp.Regexp{
    model.ProviderOpenAI:     regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`),
    model.ProviderAnthropic:  regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-]{95,}`),
    model.ProviderGoogle:     regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
    model.ProviderOpenRouter: regexp.MustCompile(`sk-or-v1-[a-zA-Z0-9]{64}`),
    model.ProviderMoonshot:   regexp.MustCompile(`sk-[a-zA-Z0-9]{48,}`),
    model.ProviderYourNew:    regexp.MustCompile(`your-key-pattern`),  // Add pattern here
}
```

### 5. Add Search Query (Optional)

Add a default search query in `internal/controllers/scraper-config-controller.go`:

```go
{
    QueryPattern: `"your-key-prefix" extension:env`,
    Provider:     model.ProviderYourNew,
    Enabled:      true,
    CreatedAt:    time.Now(),
},
```

## Advanced: Credits Support

If your provider has a credits/usage API endpoint (like OpenRouter), you can return credits information:

```go
func (v *YourNewValidator) Validate(keyValue string, correlationID string) (string, map[string]interface{}, error) {
    // ... make API call ...
    
    // Parse credits from response
    credits := map[string]interface{}{
        "total_credits": totalCredits,
        "total_usage":   totalUsage,
        "checked_at":    time.Now(),
    }
    
    // Determine status based on credits
    status := model.StatusValid
    if totalCredits <= 0 {
        status = model.StatusValidNoCredits
    }
    
    return status, credits, nil
}
```

## BaseValidator Features

The `BaseValidator` provides these helper methods:

- **ExecuteRequestWithRetry**: Automatically retries on 5xx errors (3 attempts with 2s delay)
- **DetermineStatusFromResponse**: Maps HTTP status codes to key statuses
- **logResponse**: Logs detailed response info in debug mode

## Status Codes

The validator should return one of these statuses:

- `model.StatusValid`: Key is valid and has credits
- `model.StatusValidNoCredits`: Key is valid but has no credits remaining
- `model.StatusInvalid`: Key is invalid (401, 403, 429, 400)
- `model.StatusError`: Temporary error or unknown issue

## Testing

Enable debug mode to see detailed API responses:

```bash
export VERIFIER_DEBUG=true
```

This will log:
- Request URLs
- Response status codes
- Response headers
- Response bodies (truncated if > 10KB)

## Example: Moonshot Validator

See `validators/moonshot.go` for a complete example of a new provider implementation.
