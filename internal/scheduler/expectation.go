package scheduler

import (
	"fmt"

	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/model"
)

type expectation struct {
	doMatch    func(*connector.Event) bool
	onOccurred func(*connector.Event) error
	id         uint32
}

type planElement struct {
	do      func(*connector.Event) error
	isValid func(*connector.Event) bool
	after   func(*connector.Event) error
}

func getDeletePodPlanElement(scheduler *Scheduler, pod *model.Pod) *planElement {
	return &planElement{
		do: func(event *connector.Event) error {
			if scheduler.clusterState.NumberOfRunningPods[pod.Deployment.Id] <= 1 && pod.Status == model.RUNNING {
				return fmt.Errorf("should not delete this pod because this is the only one running")
			}

			ok, err := scheduler.connector.DeletePod(pod)
			if !ok {
				return fmt.Errorf("there is no pod %d in k8s, so can't delete", pod.Id)
			}
			if err != nil {
				return err
			}

			log.Info().Msgf("--- pod deletion pod %d", pod.Id)
			return nil
		},
		isValid: func(event *connector.Event) bool {
			return event.Pod.Id == pod.Id && event.EventType == connector.POD_DELETED
		},
		after: func(event *connector.Event) error {
			ok := scheduler.clusterState.RemovePod(pod)
			if !ok {
				return fmt.Errorf("there is no pod %d in cluster state, so can't delete", pod.Id)
			}

			log.Info().Msgf("--- pod deletion verified pod %d", pod.Id)
			return nil
		},
	}
}

func getCreatePodPlanElement(scheduler *Scheduler, deployment *model.Deployment) *planElement {
	return &planElement{
		do: func(event *connector.Event) error {
			log.Info().Msgf("--- pod creation deployment %d", deployment.Id)
			return nil
		},
		isValid: func(event *connector.Event) bool {
			return event.Pod.Deployment.Id == deployment.Id && event.EventType == connector.POD_CREATED
		},
		after: func(event *connector.Event) error {
			log.Info().Msgf("--- pod creation verified deployment %d", deployment.Id)
			return nil
		},
	}
}

func getMigrateBindPodPlanElement(scheduler *Scheduler, deployment *model.Deployment, node *model.Node) *planElement {
	return &planElement{
		do: func(event *connector.Event) error {
			err := scheduler.connector.Deploy(event.Pod, node)
			if err != nil {
				return err
			}

			log.Info().Msgf("--- migrate binding deployment %d on node %d", deployment.Id, node.Id)
			return nil
		},
		isValid: func(event *connector.Event) bool {
			return event.Pod.Deployment.Id == deployment.Id && event.EventType == connector.POD_CHANGED && event.Node.Id == node.Id
		},
		after: func(event *connector.Event) error {
			var err error

			if _, ok := scheduler.clusterState.NodeResourcesUsed[node.Id]; ok {
				err = scheduler.clusterState.DeployEdge(event.Pod, event.Node)
			} else {
				scheduler.clusterState.DeployCloud(event.Pod)
			}
			if err != nil {
				return err
			}

			log.Info().Msgf("--- migrate binding verified deployment %d on node %d", deployment.Id, node.Id)
			return nil
		},
	}
}

func getBindPodPlanElement(scheduler *Scheduler, pod *model.Pod, node *model.Node) *planElement {
	return &planElement{
		do: func(event *connector.Event) error {
			err := scheduler.connector.Deploy(pod, node)
			if err != nil {
				return err
			}

			log.Info().Msgf("--- binding pod %d on node %d", pod.Id, node.Id)
			return nil
		},
		isValid: func(event *connector.Event) bool {
			if event.Node != nil {
				log.Info().Msgf("got %d %v %d", event.Pod.Id, event.EventType, event.Node.Id)
				log.Info().Msgf("wanted %d %v %d", pod.Id, connector.POD_CHANGED, node.Id)
			}
			return event.Pod.Id == pod.Id && event.EventType == connector.POD_CHANGED && event.Node.Id == node.Id
		},
		after: func(event *connector.Event) error {
			var err error
			if _, ok := scheduler.clusterState.NodeResourcesUsed[node.Id]; ok {
				err = scheduler.clusterState.DeployEdge(pod, event.Node)
			} else {
				scheduler.clusterState.DeployCloud(pod)
			}
			if err != nil {
				return err
			}

			log.Info().Msgf("--- binding verified pod %d on node %d", pod.Id, node.Id)
			return nil
		},
	}
}
