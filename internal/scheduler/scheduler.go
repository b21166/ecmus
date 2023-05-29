package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/logging"
)

var log = logging.Get()

type migrationPhase int

// migration phases:
const (
	POD_DELETION migrationPhase = iota
	POD_RECREATION
	POD_ALLOCATION
)

type migrationInfo struct {
	phase  migrationPhase
	target *model.Node
}

type Scheduler struct {
	clusterState *model.ClusterState
	connector    connector.Connector

	migrations map[int]*migrationInfo
}

type SchedulerBridge struct {
	ClusterStateRequestStream chan<- struct{}
	ClusterStateStream        <-chan *model.ClusterState
}

func New(clusterState *model.ClusterState, connector connector.Connector) (*Scheduler, error) {
	return &Scheduler{
		clusterState: clusterState,
		connector:    connector,
		migrations:   make(map[int]*migrationInfo),
	}, nil
}

func (scheduler *Scheduler) Start() error {
	log.Info().Msg("starting the scheduler...")

	if err := scheduler.connector.FindNodes(); err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not find nodes")
	}

	if err := scheduler.connector.FindDeployments(); err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not find deployments")
	}

	log.Info().Msg("scheduler started successfully.")

	return nil
}

func (scheduler *Scheduler) handleEvent(event *connector.Event) {
	log.Info().Msgf(
		"got an event:\n%v",
		event,
	)

	pod := event.Pod

	switch event.EventType {
	case connector.POD_CREATED:
		// The pod is created, it has two meanings:
		// 1- A new replica for a deployment is set.
		// 2- The migration is entering its second phase.

		var target *model.Node

		if info, ok := scheduler.migrations[pod.Deployment.Id]; ok {
			if info.phase == POD_RECREATION {
				log.Info().Msg("the pod was created during migration")

				info.phase++
				target = info.target
			} else {
				log.Warn().Msgf(
					"A new pod from deployment %d is created in an unwanted state of migration %v",
					pod.Deployment.Id,
					info.phase,
				)

				// leave it to alg to choose.
				target = nil
			} // leave it to alg to choose.
		} else {
			// leave it to alg to choose.
			target = nil
		}

		if target == nil {
			// TODO use alg
			targets := scheduler.clusterState.Edge.Config.Nodes
			target = targets[rand.Intn(len(targets))]
		}

		if err := scheduler.connector.Deploy(pod, target); err != nil {
			log.Err(err).Msgf(
				"could not deploy pod %d in connector, the pod is going to be ignored",
				pod.Id,
			)

			return
		}

		if err := scheduler.clusterState.DeployEdge(pod, target); err != nil {
			log.Err(err).Msgf(
				"could not deploy pod %d in cluster, the pod is going to be ignored",
				pod.Id,
			)

			if ok, err := scheduler.connector.DeletePod(pod); ok && err != nil {
				log.Warn().Msgf("tried to remove the pod from connecter, but couldn't.")
			}

			return
		}

	case connector.POD_CHANGED:
		// Sync pod nodes
		if pod.Node != event.Node {
			log.Error().Msgf(
				"pod changed to a unwanted node, wanted %d, got %d",
				pod.Node.Id,
				event.Node.Id,
			)

			if pod.Node != nil {
				scheduler.clusterState.RemovePod(pod)
			}

			isCloud := false
			for _, cloud_node := range scheduler.clusterState.Cloud.Nodes {
				if cloud_node.Id == event.Node.Id {
					isCloud = true
					break
				}
			}

			if isCloud {
				scheduler.clusterState.DeployCloud(pod)
			} else {
				scheduler.clusterState.DeployEdge(pod, event.Node)
			}
		} else {
			log.Info().Msg("the pod is scheduled on the expected node")

			if info, ok := scheduler.migrations[pod.Deployment.Id]; ok {
				if info.phase == POD_ALLOCATION {
					log.Info().Msg("the pod migration is done")

					log.Info().Msgf(
						"Pod %d migration is done.",
						pod.Id,
					)
					delete(scheduler.migrations, pod.Id)
				} else {
					log.Warn().Msgf(
						"Pod %d from deployment %d is in wrong phase of migrations",
						pod.Id,
						pod.Deployment.Id,
					)
				}
			}
		}

		// Sync pod states.
		if pod.Status != event.Status {
			pod.Status = event.Status

			if pod.Status == model.FINISHED {
				log.Info().Msgf("pod %d has been finished", pod.Id)

				// Remove it from, because it don't get resources anymore.
				if pod.Node != nil {
					scheduler.clusterState.RemovePod(pod)
				}
			}
		}

	case connector.POD_DELETED:
		log.Info().Msgf("pod %d has been deleted", event.Pod.Id)
		scheduler.clusterState.RemovePod(event.Pod)
	}
}

func (scheduler *Scheduler) calcState() {
	// TODO use alg
	// TODO choose a random migration
}

func (scheduler *Scheduler) Run(ctx context.Context) (SchedulerBridge, error) {
	log.Info().Msg("scheduler is running...")

	eventStream, err := scheduler.connector.WatchSchedulingEvents()
	if err != nil {
		log.Err(err).Send()

		return SchedulerBridge{}, fmt.Errorf("could not start watching scheduling events")
	}
	log.Info().Msg("got event watcher from connector")

	ticker := time.NewTicker(time.Duration(config.SchedulerGeneralConfig.DaemonPeriodDuration) * time.Millisecond)
	clusterStateRequestStream := make(chan struct{})
	clusterStateStream := make(chan *model.ClusterState, 1024)

	go func() {
	scheduler_live:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				break scheduler_live
			case event := <-eventStream:
				scheduler.handleEvent(event)
			case <-ticker.C:
				scheduler.calcState()
			case <-clusterStateRequestStream:
				clusterStateStream <- scheduler.clusterState // TODO clone
			}
		}
	}()
	log.Info().Msg("set up scheduler's main life cycle")

	return SchedulerBridge{
		ClusterStateRequestStream: clusterStateRequestStream,
		ClusterStateStream:        clusterStateStream,
	}, nil
}
