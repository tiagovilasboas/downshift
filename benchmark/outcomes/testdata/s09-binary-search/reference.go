package task

func BinarySearch(xs []int, target int) int {
	lo, hi := 0, len(xs)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		switch {
		case xs[mid] == target:
			return mid
		case xs[mid] < target:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	return -1
}
