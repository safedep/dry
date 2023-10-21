package utils

// FindInSlice finds an item in a slice and returns the
// index. It will return -1 if not found
func FindInSlice[T comparable](items []T, x T) int {
	for idx, value := range items {
		if value == x {
			return idx
		}
	}

	return -1
}

func FindAnyWith[T any](items []T, matchFn func(item *T) bool) *T {
	var t *T

	for _, item := range items {
		if matchFn(&item) {
			t = &item
			break
		}
	}

	return t
}
