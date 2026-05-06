package github

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimitSnapshot struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

type RateLimiter struct {
	mu   sync.Mutex
	snap RateLimitSnapshot
}

func NewRateLimiter() *RateLimiter { return &RateLimiter{} }

func (r *RateLimiter) Update(h http.Header) {
	limit, _ := strconv.Atoi(h.Get("X-RateLimit-Limit"))
	rem, _ := strconv.Atoi(h.Get("X-RateLimit-Remaining"))
	resetUnix, _ := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64)
	if limit == 0 && rem == 0 && resetUnix == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snap = RateLimitSnapshot{Limit: limit, Remaining: rem, Reset: time.Unix(resetUnix, 0)}
}

func (r *RateLimiter) Snapshot() RateLimitSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snap
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	s := r.snap
	r.mu.Unlock()
	if s.Limit == 0 {
		return nil
	}
	// Back off when below 10% remaining
	if s.Remaining*10 < s.Limit && time.Until(s.Reset) > 0 {
		wait := time.Until(s.Reset)
		if wait > 5*time.Minute {
			wait = 5 * time.Minute
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
