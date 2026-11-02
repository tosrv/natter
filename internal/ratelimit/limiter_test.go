package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newClock() *clock { return &clock{t: time.Unix(1_000_000, 0)} }

func TestAllowBurstThenRefill(t *testing.T) {
	c := newClock()
	l := New(20, 10*time.Second, c.now)
	for i := 0; i < 20; i++ {
		if !l.Allow("a") {
			t.Fatalf("request %d denied", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("21st request allowed")
	}
	c.add(10 * time.Second)
	for i := 0; i < 20; i++ {
		if !l.Allow("a") {
			t.Fatalf("after refill request %d denied", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("over capacity after refill allowed")
	}
}

func TestKeysIndependent(t *testing.T) {
	c := newClock()
	l := New(1, time.Minute, c.now)
	if !l.Allow("a") || l.Allow("a") {
		t.Fatal("key a wrong")
	}
	if !l.Allow("b") {
		t.Fatal("key b should be independent")
	}
}

func TestConcurrentAllow(t *testing.T) {
	c := newClock()
	l := New(20, 10*time.Second, c.now)
	var n atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("k") {
				n.Add(1)
			}
		}()
	}
	wg.Wait()
	if n.Load() != 20 {
		t.Fatalf("allowed %d, want 20", n.Load())
	}
}

func TestSweepRemovesFullBuckets(t *testing.T) {
	c := newClock()
	l := New(2, 10*time.Second, c.now)
	l.Allow("a")
	l.Allow("b")
	l.Sweep()
	if len(l.buckets) != 2 {
		t.Fatalf("non-full buckets swept: %d", len(l.buckets))
	}
	c.add(10 * time.Second)
	l.Sweep()
	if len(l.buckets) != 0 {
		t.Fatalf("full buckets kept: %d", len(l.buckets))
	}
	if !l.Allow("a") {
		t.Fatal("allow after sweep")
	}
}
