package alg

import (
	"math"
	"sort"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
)

func SuggestCloudToEdge(clusterState *model.ClusterState) []*model.Pod {
	qosResult, err := CalcNumberOfQosSatisfactions(clusterState.Edge.Config, clusterState.Cloud.Pods, clusterState.Edge.Pods, nil, nil)
	if err != nil {
		log.Err(err).Send()

		return nil
	}

	maximumResources := clusterState.Edge.Config.GetMaximumResources()

	scoreOfMigratingPod := func(pod *model.Pod) float64 {
		var score float64
		fragmentation := utils.CalcDeFragmentation(pod.Deployment.ResourcesRequired, maximumResources)
		info := qosResult.DeploymentsQoS[pod.Deployment.Id]
		score = QoS(
			float64(info.NumberOfPodOnEdge+1)/float64(info.NumberOfPods), pod.Deployment.EdgeShare,
		) - QoS(
			float64(info.NumberOfPodOnEdge)/float64(info.NumberOfPods), pod.Deployment.EdgeShare,
		)
		score /= fragmentation

		if clusterState.NumberOfRunningPods[pod.Deployment.Id] == 1 {
			score = math.Inf(-1)
		}

		return score
	}

	availableResources := utils.SubVec(clusterState.Edge.Config.Resources, clusterState.Edge.UsedResources)

	candidPods := make([]*model.Pod, len(clusterState.Cloud.Pods))
	copy(candidPods, clusterState.Cloud.Pods)

	var ret []*model.Pod

	for i := 0; i < len(candidPods) && len(ret) < config.SchedulerGeneralConfig.MaximumCloudOffload; i++ {
		sort.Sort(&ReverseSorter[model.Pod]{
			objects: candidPods[i:],
			by:      scoreOfMigratingPod,
		})

		pod := candidPods[i]
		if scoreOfMigratingPod(pod) < 0 {
			break
		}

		if utils.LEThan(pod.Deployment.ResourcesRequired, availableResources) {
			ret = append(ret, pod)

			utils.SSubVec(availableResources, pod.Deployment.ResourcesRequired)
			qosResult.DeploymentsQoS[pod.Deployment.Id].NumberOfPodOnEdge += 1
		}
	}

	return ret
}

func SuggestReorder(clusterState *model.ClusterState) model.ReorderSuggestion {
	cloudSuggestedPods := SuggestCloudToEdge(clusterState)

	return model.ReorderSuggestion{
		CloudToEdgePods: cloudSuggestedPods,
		Decision:        MakeDecisionForNewPods(clusterState, cloudSuggestedPods, true),
	}
}
