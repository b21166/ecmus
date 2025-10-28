package utils

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

func CalcDeFragmentation(resources *mat.VecDense, nodeResources *mat.VecDense) float64 {
	var ret float64
	ret = 1
	numberOfZeros := 0

	for i := 0; i < resources.Len(); i++ {
		if math.Abs(resources.AtVec(i)) < 1e-10 {
			numberOfZeros++
		}

		norm := resources.AtVec(i) / nodeResources.AtVec(i)
		ret *= norm
	}

	if numberOfZeros == resources.Len() {
		ret = 0.1
	}

	return ret
}
