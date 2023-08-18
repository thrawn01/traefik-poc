package traefik_poc

import (
	"sync"
	"sync/atomic"
)

func NewConnLimiter(maxSizeMB int64) *Limiter {
	return &Limiter{
		max: maxSizeMB << 20,
	}
}

// Limiter is a simple instance-wide limiter to limit the number of in-flight messages being
// processed. We limit by message size to avoid pulling in too many messages during slow downstream
// write performances, which historically have caused instance OOMs & HTTP 502s to the customer.
type Limiter struct {
	max int64 // Max number of bytes allowed in-flight
	cur int64 // Current count of bytes of in-flight messages
	mu  sync.Mutex
}

func (c *Limiter) IsOverLimit() bool {
	if atomic.LoadInt64(&c.cur) > c.max {
		return true
	}
	return false
}

func (c *Limiter) Current() int64 {
	return atomic.LoadInt64(&c.cur)
}

func (c *Limiter) Increment(size int64) {
	atomic.AddInt64(&c.cur, size)
}

func (c *Limiter) Decrement(size int64) {
	atomic.AddInt64(&c.cur, -size)
}
