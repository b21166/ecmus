package utils

import "gonum.org/v1/gonum/mat"

func SubVec(a, b *mat.VecDense) *mat.VecDense {
	if a.Len() != b.Len() {
		panic("Two vectors should have the same length.")
	}

	ret := mat.NewVecDense(a.Len(), nil)
	ret.SubVec(a, b)

	return ret
}

func SSubVec(a, b *mat.VecDense) {
	a.SubVec(a, b)
}

func AddVec(a, b *mat.VecDense) *mat.VecDense {
	if a.Len() != b.Len() {
		panic("Two vectors should have the same length.")
	}

	ret := mat.NewVecDense(a.Len(), nil)
	ret.AddVec(a, b)

	return ret
}

func SAddVec(a, b *mat.VecDense) {
	a.AddVec(a, b)
}

func LEThan(a, b *mat.VecDense) bool {
	if a.Len() != b.Len() {
		panic("Two vectors should have the same length.")
	}

	for i := 0; i < a.Len(); i += 1 {
		if a.AtVec(i) > b.AtVec(i) {
			return false
		}
	}

	return true
}

func LThan(a, b *mat.VecDense) bool {
	return !LEThan(b, a)
}
