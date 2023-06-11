// Because it is a testing package, no errors are returned,
// all problems cause a panic.

package testing_tool

import (
	"fmt"

	"github.com/amsen20/ecmus/alg"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/logging"
)

var log = logging.Get()

func ApplyDecision(clusterState *model.ClusterState, decision model.DecisionForNewPods) {
	for _, pod := range decision.EdgeToCloudOffloadingPods {
		if ok := clusterState.RemovePod(pod); !ok {
			panic(fmt.Sprintf("pod %d was not on edge, but tried to be removed", pod.Id))
		}
		clusterState.DeployCloud(pod)
	}

	// TODO rearrange migrations
	for _, migration := range decision.Migrations {
		if ok := clusterState.RemovePod(migration.Pod); !ok {
			panic(fmt.Sprintf("pod %d was not on edge, but tried to be removed", migration.Pod.Id))
		}
		if err := clusterState.DeployEdge(migration.Pod, migration.Node); err != nil {
			panic(err)
		}
	}
	for _, pod := range decision.ToCloudPods {
		clusterState.DeployCloud(pod)
	}

	edgeMapping := alg.MapPodsToEdge(clusterState, decision.ToEdgePods)
	for _, pod := range decision.ToEdgePods {
		if node, ok := edgeMapping[pod.Id]; ok {
			if err := clusterState.DeployEdge(pod, node); err != nil {
				panic(err)
			}
		} else {
			log.Warn().Msgf("couldn't deploy pod %d on edge, deploying on cloud", pod.Id)
			clusterState.DeployCloud(pod)
		}
	}
}
