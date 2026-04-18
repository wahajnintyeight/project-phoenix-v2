package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all service configuration
type Config struct {
	MongoURI           string
	MongoDBName        string
	GitHubTokens       []string
	ScraperPort        string
	WorkerPort         string
	ScrapingInterval   time.Duration
	ValidationInterval time.Duration
	RateLimitDelay     time.Duration
	MaxValidKeys       int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		MongoURI:    os.Getenv("MONGO_URI"),
		MongoDBName: os.Getenv("MONGO_DB_NAME"),
		ScraperPort: os.Getenv("SCRAPER_SERVICE_PORT"),
		WorkerPort:  os.Getenv("WORKER_SERVICE_PORT"),
	}

	// Parse GitHub tokens
	githubTokens := os.Getenv("GITHUB_API_TOKEN")
	if githubTokens != "" {
		parts := strings.Split(githubTokens, ",")
		config.GitHubTokens = make([]string, 0, len(parts))
		for _, token := range parts {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			config.GitHubTokens = append(config.GitHubTokens, token)
		}
	}

	// Parse scraping interval
	scrapingIntervalStr := os.Getenv("SCRAPING_INTERVAL_MINUTES")
	if scrapingIntervalStr == "" {
		scrapingIntervalStr = "20" // Default
	}
	scrapingMinutes, err := strconv.Atoi(scrapingIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SCRAPING_INTERVAL_MINUTES: %w", err)
	}
	config.ScrapingInterval = time.Duration(scrapingMinutes) * time.Minute

	// Parse validation interval
	validationIntervalStr := os.Getenv("VALIDATION_INTERVAL_MINUTES")
	if validationIntervalStr == "" {
		validationIntervalStr = "60" // Default
	}
	validationMinutes, err := strconv.Atoi(validationIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid VALIDATION_INTERVAL_MINUTES: %w", err)
	}
	config.ValidationInterval = time.Duration(validationMinutes) * time.Minute

	// Parse rate limit delay
	rateLimitDelayStr := os.Getenv("GITHUB_RATE_LIMIT_DELAY_SECONDS")
	if rateLimitDelayStr == "" {
		rateLimitDelayStr = "5" // Default
	}
	rateLimitSeconds, err := strconv.Atoi(rateLimitDelayStr)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_RATE_LIMIT_DELAY_SECONDS: %w", err)
	}
	config.RateLimitDelay = time.Duration(rateLimitSeconds) * time.Second

	// Parse max valid keys
	maxValidKeysStr := os.Getenv("MAX_VALID_KEYS")
	if maxValidKeysStr == "" {
		maxValidKeysStr = "50" // Default
	}
	maxValidKeys, err := strconv.Atoi(maxValidKeysStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_VALID_KEYS: %w", err)
	}
	config.MaxValidKeys = maxValidKeys

	return config, nil
}

// Validate validates all configuration values
func (c *Config) Validate() error {
	// Check required fields
	if c.MongoURI == "" {
		return errors.New("MONGO_URI is required")
	}
	if c.MongoDBName == "" {
		return errors.New("MONGO_DB_NAME is required")
	}
	if len(c.GitHubTokens) == 0 || c.GitHubTokens[0] == "" {
		return errors.New("GITHUB_API_TOKEN is required")
	}

	// Validate MongoDB URI format
	if _, err := url.Parse(c.MongoURI); err != nil {
		return fmt.Errorf("invalid MONGO_URI format: %w", err)
	}

	// Validate ports are numeric
	if c.ScraperPort != "" {
		if _, err := strconv.Atoi(c.ScraperPort); err != nil {
			return fmt.Errorf("invalid SCRAPER_SERVICE_PORT: must be numeric")
		}
	}
	if c.WorkerPort != "" {
		if _, err := strconv.Atoi(c.WorkerPort); err != nil {
			return fmt.Errorf("invalid WORKER_SERVICE_PORT: must be numeric")
		}
	}

	// Validate intervals are positive
	if c.ScrapingInterval <= 0 {
		return errors.New("SCRAPING_INTERVAL_MINUTES must be positive")
	}
	if c.ValidationInterval <= 0 {
		return errors.New("VALIDATION_INTERVAL_MINUTES must be positive")
	}
	if c.RateLimitDelay <= 0 {
		return errors.New("GITHUB_RATE_LIMIT_DELAY_SECONDS must be positive")
	}
	if c.MaxValidKeys <= 0 {
		return errors.New("MAX_VALID_KEYS must be positive")
	}

	return nil
}
