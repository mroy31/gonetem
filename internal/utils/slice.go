package utils

func SliceFilter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func SliceFilterCount[T any](ss []T, test func(T) bool) int {
	count := 0
	for _, s := range ss {
		if test(s) {
			count += 1
		}
	}
	return count
}
