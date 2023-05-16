package alg

import (
	"math"

	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"gonum.org/v1/gonum/mat"
)

func MakeDecisionForNewPods(c *model.ClusterState, newPods []*model.Pod) model.DecisionForNewPods {
	bestDecision := model.DecisionForNewPods{
		Score: math.Inf(-1),
	}

	for edgeSubSetNewPodsMask := 0; edgeSubSetNewPodsMask < (1 << len(newPods)); edgeSubSetNewPodsMask++ {
		edgeNewPods := make([]*model.Pod, 0)
		cloudNewPods := make([]*model.Pod, 0)

		for i, pod := range newPods {
			if edgeSubSetNewPodsMask&(1<<i) > 0 {
				edgeNewPods = append(edgeNewPods, pod)
			} else {
				cloudNewPods = append(cloudNewPods, pod)
			}
		}

		leastResourceNeeded := mat.NewVecDense(2, nil)
		for _, pod := range edgeNewPods {
			utils.SAddVec(leastResourceNeeded, pod.Deployment.ResourcesRequired)
		}

		// TODO read it from candidate list
		freeEdgeSol, err := CalcState(c, leastResourceNeeded)
		if err != nil {
			continue
		}

		currentDecision := model.DecisionForNewPods{
			Score:                     -freeEdgeSol.Score,
			EdgeToCloudOffloadingPods: freeEdgeSol.FreedPods,
			ToEdgePods:                edgeNewPods,
			ToCloudPods:               cloudNewPods,
			Migrations:                freeEdgeSol.Migrations,
		}

		for _, pod := range edgeNewPods {
			currentDecision.Score += WEIGHT_COEFFICIENT * pod.Deployment.Weight
		}

		if currentDecision.Score > bestDecision.Score {
			bestDecision = currentDecision
		}
	}

	return bestDecision
}
