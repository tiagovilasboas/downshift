package task

import (
	"context"
	"fmt"
	"time"
)

func Retry(ctx context.Context, attempts int, base time.Duration, sleep func(time.Duration), fn func() error) error {
	var last error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if last = fn(); last == nil {
			return nil
		}
		if i < attempts-1 {
			sleep(base << i)
		}
	}
	return fmt.Errorf("after %d attempts: %w", attempts, last)
}
