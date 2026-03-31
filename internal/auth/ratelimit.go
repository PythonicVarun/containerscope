package auth

import (
	"net/http"
	"sync"
	"time"
)

const (
	maxLoginAttempts = 5
	lockoutDuration  = 15 * time.Minute
	cleanupInterval  = 5 * time.Minute
)

type loginAttempt struct {
	count    int
	firstTry time.Time
	lockedAt time.Time
	isLocked bool
}

type RateLimiter struct {
	attempts map[string]*loginAttempt
	mu       sync.Mutex
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*loginAttempt),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	attempt, exists := rl.attempts[ip]
	if !exists {
		return true
	}

	if attempt.isLocked {
		if time.Since(attempt.lockedAt) > lockoutDuration {
			delete(rl.attempts, ip)
			return true
		}
		return false
	}

	return true
}

func (rl *RateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	attempt, exists := rl.attempts[ip]
	if !exists {
		rl.attempts[ip] = &loginAttempt{
			count:    1,
			firstTry: time.Now(),
		}
		return
	}

	attempt.count++
	if attempt.count >= maxLoginAttempts {
		attempt.isLocked = true
		attempt.lockedAt = time.Now()
	}
}

func (rl *RateLimiter) RecordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

func (rl *RateLimiter) IsLocked(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	attempt, exists := rl.attempts[ip]
	if !exists || !attempt.isLocked {
		return false, 0
	}

	remaining := lockoutDuration - time.Since(attempt.lockedAt)
	if remaining <= 0 {
		delete(rl.attempts, ip)
		return false, 0
	}

	return true, remaining
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, attempt := range rl.attempts {
		if attempt.isLocked && now.Sub(attempt.lockedAt) > lockoutDuration {
			delete(rl.attempts, ip)
		} else if !attempt.isLocked && now.Sub(attempt.firstTry) > lockoutDuration {
			delete(rl.attempts, ip)
		}
	}
}

func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
