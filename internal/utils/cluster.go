package utils

import "gonum.org/v1/gonum/mat"

func CalcDeFragmentation(resources *mat.VecDense, nodeResources *mat.VecDense) float64 {
	var ret float64
	ret = 1

	for i := 0; i < resources.Len(); i++ {
		norm := resources.AtVec(i) / nodeResources.AtVec(i)
		ret *= norm
	}

	return ret
}
