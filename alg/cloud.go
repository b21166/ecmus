package alg

import (
	"sort"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
)

func SuggestCloudToEdge(clusterState *model.ClusterState) model.CloudSuggestion {
	qosResult, err := CalcNumberOfQosSatisfactions(clusterState.Edge.Config, clusterState.Cloud.Pods, clusterState.Edge.Pods, nil, nil)
	if err != nil {
		log.Err(err).Send()

		return model.CloudSuggestion{}
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

		return score
	}

	availableResources := utils.SubVec(clusterState.Edge.Config.Resources, clusterState.Edge.UsedResources)

	candidPods := make([]*model.Pod, len(clusterState.Cloud.Pods))
	copy(candidPods, clusterState.Cloud.Pods)

	ret := model.CloudSuggestion{}

	for i := 0; i < len(candidPods) && len(ret.Migrations) < config.SchedulerGeneralConfig.MaximumCloudOffload; i++ {
		sort.Sort(&ReverseSorter[model.Pod]{
			objects: candidPods[i:],
			by:      scoreOfMigratingPod,
		})

		pod := candidPods[i]
		if utils.LEThan(pod.Deployment.ResourcesRequired, availableResources) {
			ret.Migrations = append(ret.Migrations, pod)

			utils.SSubVec(availableResources, pod.Deployment.ResourcesRequired)
			qosResult.DeploymentsQoS[pod.Deployment.Id].NumberOfPodOnEdge += 1
			break
		}
	}

	return ret
}
