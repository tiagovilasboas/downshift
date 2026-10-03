package task

import (
	"context"
	"sync"
)

func ParallelMap(ctx context.Context, inputs []int, workers int, fn func(context.Context, int) (int, error)) ([]int, error) {
	if workers < 1 {
		workers = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]int, len(inputs))
	jobs := make(chan int)
	var once sync.Once
	var firstErr error
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				v, err := fn(ctx, inputs[i])
				if err != nil {
					once.Do(func() { firstErr = err; cancel() })
					continue
				}
				out[i] = v
			}
		}()
	}
feed:
	for i := range inputs {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
