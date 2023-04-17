package utils

import "gonum.org/v1/gonum/mat"

func CalcDeFragmentation(resources *mat.VecDense, maxResources *mat.VecDense) float64 {
	var ret float64

	for i := 0; i < resources.Len(); i++ {
		norm := resources.AtVec(i) / maxResources.AtVec(i)
		norm *= norm
		ret += norm
	}

	return ret
}
