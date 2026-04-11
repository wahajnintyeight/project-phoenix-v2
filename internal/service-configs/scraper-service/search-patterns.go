package scraperconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SearchPattern represents a single search query pattern
type SearchPattern struct {
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

// ProviderPatterns represents all patterns for a specific provider
type ProviderPatterns struct {
	Provider string          `json:"provider"`
	Enabled  bool            `json:"enabled"`
	Queries  []SearchPattern `json:"queries"`
}

// SearchPatternsConfig represents the entire search patterns configuration
type SearchPatternsConfig struct {
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Patterns    []ProviderPatterns `json:"patterns"`
	Notes       []string           `json:"notes"`
}

// LoadSearchPatterns loads the search patterns from the JSON configuration file
func LoadSearchPatterns() (*SearchPatternsConfig, error) {
	// Try to load from the service-configs directory
	configPath := filepath.Join("internal", "service-configs", "scraper-service", "search-patterns.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read search patterns config: %w", err)
	}

	var config SearchPatternsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse search patterns config: %w", err)
	}

	return &config, nil
}

// GetEnabledPatterns returns all enabled search patterns across all providers
func (c *SearchPatternsConfig) GetEnabledPatterns() []SearchPattern {
	var patterns []SearchPattern

	for _, provider := range c.Patterns {
		if provider.Enabled {
			patterns = append(patterns, provider.Queries...)
		}
	}

	return patterns
}

// GetPatternsByProvider returns all patterns for a specific provider
func (c *SearchPatternsConfig) GetPatternsByProvider(providerName string) []SearchPattern {
	for _, provider := range c.Patterns {
		if provider.Provider == providerName && provider.Enabled {
			return provider.Queries
		}
	}

	return nil
}

// GetAllProviders returns a list of all provider names
func (c *SearchPatternsConfig) GetAllProviders() []string {
	providers := make([]string, 0, len(c.Patterns))
	for _, provider := range c.Patterns {
		providers = append(providers, provider.Provider)
	}
	return providers
}

// CountEnabledQueries returns the total number of enabled queries
func (c *SearchPatternsConfig) CountEnabledQueries() int {
	count := 0
	for _, provider := range c.Patterns {
		if provider.Enabled {
			count += len(provider.Queries)
		}
	}
	return count
}
