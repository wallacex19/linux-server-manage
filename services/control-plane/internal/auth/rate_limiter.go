package auth

import (
	"sync"
	"time"
)

type attemptTracker struct {
	count       int
	lockUntil   time.Time
	lastAttempt time.Time
}

type LoginRateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*attemptTracker
	maxAttempts int
	lockoutTime time.Duration
}

func NewLoginRateLimiter(maxAttempts int, lockoutTime time.Duration) *LoginRateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if lockoutTime <= 0 {
		lockoutTime = 5 * time.Minute
	}
	return &LoginRateLimiter{
		attempts:    make(map[string]*attemptTracker),
		maxAttempts: maxAttempts,
		lockoutTime: lockoutTime,
	}
}

func (r *LoginRateLimiter) IsAllowed(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	tracker, exists := r.attempts[key]
	if !exists {
		return true
	}

	now := time.Now().UTC()
	if !tracker.lockUntil.IsZero() {
		if now.Before(tracker.lockUntil) {
			return false
		}
		// Lockout expirou: reseta contador de tentativas
		tracker.count = 0
		tracker.lockUntil = time.Time{}
		return true
	}

	// Reseta se a última tentativa foi há mais de 10 minutos
	if now.Sub(tracker.lastAttempt) > 10*time.Minute {
		delete(r.attempts, key)
		return true
	}

	return tracker.count < r.maxAttempts
}

func (r *LoginRateLimiter) RecordFailure(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	tracker, exists := r.attempts[key]
	if !exists {
		tracker = &attemptTracker{}
		r.attempts[key] = tracker
	}

	tracker.lastAttempt = now
	tracker.count++

	if tracker.count >= r.maxAttempts {
		tracker.lockUntil = now.Add(r.lockoutTime)
	}
}

func (r *LoginRateLimiter) RecordSuccess(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
}
