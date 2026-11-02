// Package ratelimit provides an in-memory, per-key token bucket limiter.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter allows up to limit events per window per key, refilling continuously.
type Limiter struct {
	mu      sync.Mutex
	limit   float64
	rate    float64 // tokens per nanosecond
	now     func() time.Time
	buckets map[string]*bucket
}

// New creates a Limiter with capacity limit and refill rate limit/window.
func New(limit int, window time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		limit:   float64(limit),
		rate:    float64(limit) / float64(window),
		now:     now,
		buckets: make(map[string]*bucket),
	}
}

func (l *Limiter) refill(b *bucket, t time.Time) {
	if el := t.Sub(b.last); el > 0 {
		b.tokens += float64(el) * l.rate
		if b.tokens > l.limit {
			b.tokens = l.limit
		}
		b.last = t
	}
}

// Allow consumes one token for key, reporting whether it was available.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.limit, last: t}
		l.buckets[key] = b
	} else {
		l.refill(b, t)
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Sweep removes buckets that have refilled to capacity (idle keys).
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.now()
	for k, b := range l.buckets {
		l.refill(b, t)
		if b.tokens >= l.limit {
			delete(l.buckets, k)
		}
	}
}
