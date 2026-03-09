package auth

import (
	"sync"
	"time"
)

type loginAttempt struct {
	count       int
	windowStart time.Time
}

// LoginRateLimiter tracks login attempts per key (IP address)
// using a simple fixed-window counter.
type LoginRateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*loginAttempt
	maxAttempts int
	window      time.Duration
	sweepCounter   int
}

// NewLoginRateLimiter creates a rate limiter that allows maxAttempts
// per key within the given window duration.
func NewLoginRateLimiter(maxAttempts int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxAttempts: maxAttempts,
		window:      window,
	}
}

// Allow checks if the key is allowed to make a login attempt.
// Returns true if allowed, false if rate limited.
func (rl *LoginRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.sweepCounter++
	if rl.sweepCounter%100 == 0 {
		now := time.Now()
		for k, a := range rl.attempts {
			if now.Sub(a.windowStart) > rl.window {
				delete(rl.attempts, k)
			}
		}
	}

	now := time.Now()
	attempt, exists := rl.attempts[key]

	if !exists || now.Sub(attempt.windowStart) >= rl.window {
		rl.attempts[key] = &loginAttempt{count: 1, windowStart: now}
		return true
	}

	if attempt.count >= rl.maxAttempts {
		return false
	}

	attempt.count++
	return true
}
