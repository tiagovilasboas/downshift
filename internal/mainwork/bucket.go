// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package mainwork

// Result size buckets for a Read result. A bucket is a size class, never the
// result's content or the file it came from.
const (
	BucketUnder1K  = "<1K"
	Bucket1To4K    = "1-4K"
	Bucket4To20K   = "4-20K"
	Bucket20To100K = "20-100K"
	BucketOver100K = ">100K"
)

// Buckets lists the size classes in ascending order.
var Buckets = []string{BucketUnder1K, Bucket1To4K, Bucket4To20K, Bucket20To100K, BucketOver100K}

// SizeBucket returns the class of a result of n bytes (1K = 1024). n <= 0
// means the size is unknown and returns "": nothing is recorded.
func SizeBucket(n int) string {
	switch {
	case n <= 0:
		return ""
	case n < 1<<10:
		return BucketUnder1K
	case n < 4<<10:
		return Bucket1To4K
	case n < 20<<10:
		return Bucket4To20K
	case n <= 100<<10:
		return Bucket20To100K
	}
	return BucketOver100K
}
