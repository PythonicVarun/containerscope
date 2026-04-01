package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterLocksAfterMaxFailuresAndResetsOnSuccess(t *testing.T) {
	limiter := &RateLimiter{attempts: make(map[string]*loginAttempt)}
	ip := "203.0.113.10"

	for range maxLoginAttempts - 1 {
		limiter.RecordFailure(ip)
	}

	if !limiter.Allow(ip) {
		t.Fatal("expected requests to be allowed before reaching the max attempts")
	}

	if locked, _ := limiter.IsLocked(ip); locked {
		t.Fatal("expected IP to remain unlocked before the final failure")
	}

	limiter.RecordFailure(ip)

	if limiter.Allow(ip) {
		t.Fatal("expected requests to be blocked after reaching the max attempts")
	}

	locked, remaining := limiter.IsLocked(ip)
	if !locked {
		t.Fatal("expected IP to be locked")
	}
	if remaining <= 0 {
		t.Fatalf("expected positive lockout duration, got %v", remaining)
	}

	limiter.RecordSuccess(ip)

	if !limiter.Allow(ip) {
		t.Fatal("expected RecordSuccess to clear the lockout")
	}

	if locked, _ := limiter.IsLocked(ip); locked {
		t.Fatal("expected IP to be unlocked after RecordSuccess")
	}
}

func TestRateLimiterCleanupRemovesExpiredAttempts(t *testing.T) {
	limiter := &RateLimiter{
		attempts: map[string]*loginAttempt{
			"expired-locked": {
				count:    maxLoginAttempts,
				lockedAt: time.Now().Add(-lockoutDuration - time.Second),
				isLocked: true,
			},
			"expired-unlocked": {
				count:    1,
				firstTry: time.Now().Add(-lockoutDuration - time.Second),
			},
			"fresh": {
				count:    1,
				firstTry: time.Now(),
			},
		},
	}

	limiter.cleanup()

	if _, exists := limiter.attempts["expired-locked"]; exists {
		t.Fatal("expected expired locked attempt to be removed")
	}
	if _, exists := limiter.attempts["expired-unlocked"]; exists {
		t.Fatal("expected expired unlocked attempt to be removed")
	}
	if _, exists := limiter.attempts["fresh"]; !exists {
		t.Fatal("expected fresh attempt to remain")
	}
}

func TestGetClientIPPrefersForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.24:8080"

	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")
	if got := GetClientIP(req); got != "203.0.113.1" {
		t.Fatalf("expected X-Forwarded-For to win, got %q", got)
	}

	req.Header.Del("X-Forwarded-For")
	if got := GetClientIP(req); got != "203.0.113.2" {
		t.Fatalf("expected X-Real-IP to be used, got %q", got)
	}

	req.Header.Del("X-Real-IP")
	if got := GetClientIP(req); got != "198.51.100.24:8080" {
		t.Fatalf("expected RemoteAddr fallback, got %q", got)
	}
}
