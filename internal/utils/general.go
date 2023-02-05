package utils

import "hash/fnv"

func SliceToMap[T any](s []T, getId func(t T) int) map[int]bool {
	ret := make(map[int]bool)
	for i := 0; i < len(s); i++ {
		ret[getId(s[i])] = true
	}

	return ret
}

func Hash(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}
