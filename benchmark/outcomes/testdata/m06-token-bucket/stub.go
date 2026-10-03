package task

import (
	"time"
)

type Bucket struct{}

func NewBucket(capacity int, refillPerSec float64, now func() time.Time) *Bucket {
	panic("not implemented")
}
func (b *Bucket) Allow() bool { panic("not implemented") }
