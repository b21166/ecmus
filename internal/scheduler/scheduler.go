package scheduler

import (
	"fmt"
	"math/rand"

	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/rs/zerolog/log"
)

type Scheduler struct {
	ClusterState *model.ClusterState
	Connector    connector.Connector
}

func (scheduler *Scheduler) Start() error {
	if err := scheduler.Connector.FindNodes(); err != nil {
		log.Err(err).Send()

		return fmt.Errorf("connector could not find nodes")
	}

	if err := scheduler.Connector.FindDeployments(); err != nil {
		log.Err(err).Send()

		return fmt.Errorf("connector could not find deployments")
	}

	return nil
}

func (scheduler *Scheduler) Run() error {
	eventStream, err := scheduler.Connector.WatchSchedulingEvents()
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not start watching scheduling events")
	}

	for event := range eventStream {
		pod := event.Pod

		switch event.EventType {
		case connector.POD_CREATED:
			targets := scheduler.ClusterState.Edge.Config.Nodes
			target := targets[rand.Intn(len(targets))]
			if err := scheduler.ClusterState.DeployEdge(pod, target); err != nil {
				log.Err(err).Msgf("could not deploy pod %d in cluster", pod.Id)

				continue
			}

			if err := scheduler.Connector.Deploy(pod); err != nil {
				log.Err(err).Msgf("could not deploy pod %d in connector", pod.Id)

				continue
			}
		}
	}
	return nil
}
