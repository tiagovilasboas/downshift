package task

import (
	"context"
	"time"
)

func Retry(ctx context.Context, attempts int, base time.Duration, sleep func(time.Duration), fn func() error) error {
	panic("not implemented")
}
