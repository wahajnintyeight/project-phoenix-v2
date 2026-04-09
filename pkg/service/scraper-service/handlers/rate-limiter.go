package handlers

import (
	"log"
	"sync"
	"time"
)

// RateLimiter enforces delays between GitHub API requests
type RateLimiter struct {
	minDelay       time.Duration
	lastRequest    time.Time
	mutex          sync.Mutex
	remainingQuota int
	quotaResetTime time.Time
}

// NewRateLimiter creates a new rate limiter with the specified minimum delay
func NewRateLimiter(minDelay time.Duration) *RateLimiter {
	return &RateLimiter{
		minDelay:       minDelay,
		lastRequest:    time.Time{},
		remainingQuota: 30, // GitHub default
		quotaResetTime: time.Now().Add(time.Minute),
	}
}

// Wait enforces the minimum delay between requests
func (r *RateLimiter) Wait() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.lastRequest.IsZero() {
		elapsed := time.Since(r.lastRequest)
		if elapsed < r.minDelay {
			sleepDuration := r.minDelay - elapsed
			// log.Printf("Rate limiter: sleeping for %v", sleepDuration)
			time.Sleep(sleepDuration)
		}
	}

	r.lastRequest = time.Now()
}

// UpdateQuota updates the rate limit quota from GitHub response headers
func (r *RateLimiter) UpdateQuota(remaining int, resetTime time.Time) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.remainingQuota = remaining
	r.quotaResetTime = resetTime

	// log.Printf("Rate limit updated: %d requests remaining, resets at %v", remaining, resetTime)
}

// ShouldPause checks if the quota is below the threshold and we should pause scraping
func (r *RateLimiter) ShouldPause() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.remainingQuota < 10
}

// GetQuotaInfo returns current quota information
func (r *RateLimiter) GetQuotaInfo() (remaining int, resetTime time.Time) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.remainingQuota, r.quotaResetTime
}
