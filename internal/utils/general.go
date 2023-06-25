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

func Permutations[T any](arr []T) <-chan []T {
	var helper func([]T, int)
	res := make(chan []T)

	helper = func(arr []T, n int) {
		if n == 1 {
			tmp := make([]T, len(arr))
			copy(tmp, arr)
			res <- tmp
		} else {
			for i := 0; i < n; i++ {
				helper(arr, n-1)
				if n%2 == 1 {
					tmp := arr[i]
					arr[i] = arr[n-1]
					arr[n-1] = tmp
				} else {
					tmp := arr[0]
					arr[0] = arr[n-1]
					arr[n-1] = tmp
				}
			}
		}
	}
	go func() {
		helper(arr, len(arr))
		close(res)
	}()

	return res
}
