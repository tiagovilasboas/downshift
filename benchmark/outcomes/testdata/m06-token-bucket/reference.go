package task

import (
	"time"
)

type Bucket struct {
	cap    float64
	rate   float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

func NewBucket(capacity int, refillPerSec float64, now func() time.Time) *Bucket {
	return &Bucket{cap: float64(capacity), rate: refillPerSec, tokens: float64(capacity), last: now(), now: now}
}

func (b *Bucket) Allow() bool {
	t := b.now()
	b.tokens += t.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.cap {
		b.tokens = b.cap
	}
	b.last = t
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
