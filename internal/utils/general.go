package utils

func SliceToMap[T any](s []T, getId func(t T) int) map[int]bool {
	ret := make(map[int]bool)
	for i := 0; i < len(s); i++ {
		ret[getId(s[i])] = true
	}

	return ret
}
