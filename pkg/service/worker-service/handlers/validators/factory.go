package validators

import (
	"fmt"

	"project-phoenix/v2/internal/model"
)

// ValidatorFactory creates provider-specific validators
type ValidatorFactory struct {
	validators map[string]Validator
	debugMode  bool
}

// NewValidatorFactory creates a new validator factory
func NewValidatorFactory(debugMode bool) *ValidatorFactory {
	factory := &ValidatorFactory{
		validators: make(map[string]Validator),
		debugMode:  debugMode,
	}

	// Register all validators
	factory.registerValidators()

	return factory
}

// registerValidators registers all available validators
func (f *ValidatorFactory) registerValidators() {
	f.Register(NewOpenAIValidator(f.debugMode))
	f.Register(NewAnthropicValidator(f.debugMode))
	f.Register(NewGoogleValidator(f.debugMode))
	f.Register(NewOpenRouterValidator(f.debugMode))
	f.Register(NewMoonshotValidator(f.debugMode))
	f.Register(NewHuggingFaceValidator(f.debugMode))
	f.Register(NewDeepSeekValidator(f.debugMode))
	f.Register(NewZAIValidator(f.debugMode))
}

// Register adds a validator to the factory
func (f *ValidatorFactory) Register(validator Validator) {
	f.validators[validator.GetProviderName()] = validator
}

// GetValidator returns a validator for the specified provider
func (f *ValidatorFactory) GetValidator(provider string) (Validator, error) {
	validator, exists := f.validators[provider]
	if !exists {
		return nil, fmt.Errorf("no validator found for provider: %s", provider)
	}
	return validator, nil
}

// GetSupportedProviders returns a list of all supported providers
func (f *ValidatorFactory) GetSupportedProviders() []string {
	providers := make([]string, 0, len(f.validators))
	for provider := range f.validators {
		providers = append(providers, provider)
	}
	return providers
}

// ValidateKey validates a key using the appropriate provider validator.
// If provider is "Other", the key is tested against all registered validators.
func (f *ValidatorFactory) ValidateKey(provider, keyValue, correlationID string) (string, map[string]interface{}, error) {
	if provider == "Other" {
		return f.validateAgainstAll(keyValue, correlationID)
	}

	validator, err := f.GetValidator(provider)
	if err != nil {
		return model.StatusError, nil, err
	}

	return validator.Validate(keyValue, correlationID)
}

// validateAgainstAll tries the key against every registered provider validator.
// Returns the first valid result found, or invalid if none match.
func (f *ValidatorFactory) validateAgainstAll(keyValue, correlationID string) (string, map[string]interface{}, error) {
	for _, validator := range f.validators {
		status, credits, err := validator.Validate(keyValue, correlationID)
		if err != nil {
			continue
		}
		if status == model.StatusValid {
			return status, credits, nil
		}
	}
	return model.StatusInvalid, nil, fmt.Errorf("key did not match any known provider")
}
