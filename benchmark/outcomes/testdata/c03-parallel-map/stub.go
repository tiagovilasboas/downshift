package task

import (
	"context"
)

func ParallelMap(ctx context.Context, inputs []int, workers int, fn func(context.Context, int) (int, error)) ([]int, error) {
	panic("not implemented")
}
