package alg

import (
	"fmt"
	"math"

	"github.com/amsen20/ecmus/internal/model"
)

const (
	PRE_EDGE = 1
	NEW_EDGE = 1<<1 | 1

	PRE_CLOUD = 0
	NEW_CLOUD = 1 << 1

	CLUSTER    = 1
	STATE_TIME = 1 << 1
)

type QoSDeploymentInfo struct {
	NumberOfPodOnEdge int
	NumberOfPods      int
}

type QoSResult struct {
	Score          float64
	DeploymentsQoS map[int]*QoSDeploymentInfo
}

const (
	SATISFACTION_SCORE = 0.99
	PRE_SATISFACTION   = 0.8
)

func QoS(currentShare, promisedShare float64) float64 {
	if math.Abs(currentShare-promisedShare) < 1e-9 {
		return SATISFACTION_SCORE
	}
	if currentShare >= promisedShare {
		return SATISFACTION_SCORE + (currentShare-promisedShare)*(1-SATISFACTION_SCORE)/(1-promisedShare)
	}

	return PRE_SATISFACTION * math.Sqrt(currentShare/promisedShare)
}

// If a pod is in both pre-known and new pods, the new state
// is assumed, if a pod is both cloud and edge in at the same time,
// an error will be raised.
// This function has no effect on the input.
func CalcNumberOfQosSatisfactions(
	edgeConfig *model.EdgeConfig,
	preKnownCloudPods []*model.Pod,
	preKnownEdgePods []*model.Pod,
	newCloudPods []*model.Pod,
	newEdgePods []*model.Pod,
) (QoSResult, error) {
	deploymentPods := make(map[int]map[int]int)
	deploymentsQoS := make(map[int]*QoSDeploymentInfo)

	setState := func(state int, pods []*model.Pod) error {
		for _, pod := range pods {
			_, ok := deploymentPods[pod.Deployment.Id]
			if !ok {
				deploymentPods[pod.Deployment.Id] = make(map[int]int)
			}

			lastState, ok := deploymentPods[pod.Deployment.Id][pod.Id]
			if !ok {
				deploymentPods[pod.Deployment.Id][pod.Id] = state
				continue
			}
			change := lastState ^ state
			if change == CLUSTER {
				return fmt.Errorf("pod %d is in both cloud and edge at the same time", pod.Id)
			}

			deploymentPods[pod.Deployment.Id][pod.Id] = state
		}

		return nil
	}

	for _, iter := range []struct {
		state int
		pods  []*model.Pod
	}{
		{state: PRE_EDGE, pods: preKnownEdgePods},
		{state: PRE_CLOUD, pods: preKnownCloudPods},
		{state: NEW_EDGE, pods: newEdgePods},
		{state: NEW_CLOUD, pods: newCloudPods},
	} {
		if err := setState(iter.state, iter.pods); err != nil {
			return QoSResult{}, err
		}
	}

	var score float64

	for deploymentId, PodToState := range deploymentPods {
		numberOfPods := 0
		numberOfPodsOnEdge := 0

		for _, state := range PodToState {
			numberOfPods += 1
			if (state & CLUSTER) > 0 {
				numberOfPodsOnEdge += 1
			}
		}

		deployment, ok := edgeConfig.DeploymentIdToDeployment[deploymentId]
		if !ok {
			return QoSResult{}, fmt.Errorf("one of the deployment with id %d is not configured at first", deploymentId)
		}

		score += QoS(float64(numberOfPodsOnEdge)/float64(numberOfPods), deployment.EdgeShare)

		deploymentsQoS[deploymentId] = &QoSDeploymentInfo{
			NumberOfPodOnEdge: numberOfPodsOnEdge,
			NumberOfPods:      numberOfPods,
		}
	}

	return QoSResult{
		Score:          score,
		DeploymentsQoS: deploymentsQoS,
	}, nil
}
