package alg

import (
	"math"

	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"github.com/amsen20/ecmus/logging"
	"gonum.org/v1/gonum/mat"
)

var log = logging.Get()

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
			EdgeToCloudOffloadingPods: freeEdgeSol.FreedPods,
			ToEdgePods:                edgeNewPods,
			ToCloudPods:               cloudNewPods,
			Migrations:                freeEdgeSol.Migrations,
		}

		newCloudPods := make([]*model.Pod, 0)
		newCloudPods = append(newCloudPods, currentDecision.EdgeToCloudOffloadingPods...)
		newCloudPods = append(newCloudPods, currentDecision.ToCloudPods...)

		qosResult, err := CalcNumberOfQosSatisfactions(
			c.Edge.Config,
			c.Cloud.Pods,
			c.Edge.Pods,
			newCloudPods,
			currentDecision.ToEdgePods,
		)
		if err != nil {
			log.Err(err).Send()

			continue
		}

		currentDecision.Score = qosResult.Score

		// maxResources := c.Edge.Config.GetMaximumResources()
		// nodeResourcesRemained := c.GetNodesResourcesRemained()
		// var deFragmentation float64
		// for _, resourcesRemained := range nodeResourcesRemained {
		// 	deFragmentation += utils.CalcDeFragmentation(resourcesRemained, maxResources)
		// }

		// currentDecision.Score += FRAGMENTATION_DECISION_COEFFICIENT * deFragmentation

		if currentDecision.Score > bestDecision.Score {
			bestDecision = currentDecision
		}
	}

	return bestDecision
}
