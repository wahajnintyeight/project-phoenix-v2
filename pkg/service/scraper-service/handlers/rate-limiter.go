package handlers

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter enforces rate limits using token bucket algorithm
// This allows burst traffic while maintaining average rate limits
type RateLimiter struct {
	limiter        *rate.Limiter
	mutex          sync.Mutex
	remainingQuota int
	quotaResetTime time.Time
	pauseThreshold int
}

// NewRateLimiter creates a new token bucket rate limiter
// requestsPerMinute: how many requests allowed per minute
// burst: how many requests can be made in a burst (allows temporary spikes)
func NewRateLimiter(requestsPerMinute int, burst int) *RateLimiter {
	// Convert requests per minute to rate.Limit (requests per second)
	rps := rate.Every(time.Minute / time.Duration(requestsPerMinute))

	return &RateLimiter{
		limiter:        rate.NewLimiter(rps, burst),
		remainingQuota: requestsPerMinute,
		quotaResetTime: time.Now().Add(time.Minute),
		pauseThreshold: 10, // Pause when quota drops below this
	}
}

// Wait blocks until a token is available or context is cancelled
// This is non-blocking across goroutines - multiple goroutines can wait concurrently
func (r *RateLimiter) Wait() {
	// Use background context with no timeout - we want to wait as long as needed
	r.limiter.Wait(context.Background())
}

// WaitWithContext blocks until a token is available or context is cancelled
func (r *RateLimiter) WaitWithContext(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

// UpdateQuota updates the rate limit quota from API response headers
func (r *RateLimiter) UpdateQuota(remaining int, resetTime time.Time) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.remainingQuota = remaining
	r.quotaResetTime = resetTime
}

// ShouldPause checks if the quota is below the threshold and we should pause scraping
func (r *RateLimiter) ShouldPause() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.remainingQuota < r.pauseThreshold
}

// GetQuotaInfo returns current quota information
func (r *RateLimiter) GetQuotaInfo() (remaining int, resetTime time.Time) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.remainingQuota, r.quotaResetTime
}
