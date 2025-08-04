package ai

import (
	"context"
	"time"
)

// RateLimiter implements a token bucket rate limiter for AI API calls.
type RateLimiter struct {
	tokens  chan struct{}
	stopped chan struct{}
}

// NewRateLimiter creates a new rate limiter that allows 'limit' requests per 'window'.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	tokens := make(chan struct{}, limit)

	// Fill initial tokens
	for i := 0; i < limit; i++ {
		tokens <- struct{}{}
	}

	rl := &RateLimiter{
		tokens:  tokens,
		stopped: make(chan struct{}),
	}

	// Start background refill goroutine
	go rl.refillTokens(limit, window)

	return rl
}

// Wait blocks until a token is available or context is canceled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.stopped:
		return context.Canceled
	}
}

// Stop stops the rate limiter.
func (rl *RateLimiter) Stop() {
	close(rl.stopped)
}

func (rl *RateLimiter) refillTokens(limit int, window time.Duration) {
	ticker := time.NewTicker(window / time.Duration(limit))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
				// Token added
			default:
				// Channel full, skip this refill
			}
		case <-rl.stopped:
			return
		}
	}
}
