package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/amsen20/ecmus/alg"
	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"github.com/amsen20/ecmus/logging"
	"github.com/google/uuid"
)

var log = logging.Get()

type migrationPhase int

// migration phases:
const (
	POD_DELETION migrationPhase = iota
	POD_RECREATION
	POD_ALLOCATION
)

type Scheduler struct {
	clusterState *model.ClusterState
	connector    connector.Connector

	newPodBuffer []*model.Pod
	expectations []*expectation

	healthCheckSample *healthCheckSample
}

type SchedulerBridge struct {
	ClusterStateRequestStream chan<- struct{}
	ClusterStateStream        <-chan *model.ClusterState
	CanSchedule               chan<- struct{}
	CanSuggestCloud           chan<- struct{}
}

func New(clusterState *model.ClusterState, connector connector.Connector) (*Scheduler, error) {
	return &Scheduler{
		clusterState: clusterState,
		connector:    connector,
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

func (scheduler *Scheduler) flushExpectations(reschedule bool) {
	log.Info().Msg("flushing expectations")
	scheduler.expectations = nil
	if reschedule {
		scheduler.schedule()
	}
}

func (scheduler *Scheduler) nextExpectation() {
	if len(scheduler.expectations) > 0 {
		scheduler.expectations = scheduler.expectations[1:]
	}
	if len(scheduler.expectations) == 0 {
		scheduler.schedule()
	}
}

func (scheduler *Scheduler) handleEvent(event *connector.Event) {
	log.Info().Msgf(
		"got an event:\n%v",
		event,
	)

	pod, ok := scheduler.clusterState.PodsMap[event.Pod.Id]
	if !ok {
		return
	}
	log.Info().Msgf("%v", pod)

	podCreation := event.EventType == connector.POD_CREATED
	podStatusChange := event.EventType == connector.POD_CHANGED && pod.Node == event.Node && pod.Status != event.Status

	theSame := event.EventType == connector.POD_CHANGED && pod.Status == event.Status && pod.Node == event.Node
	theSame = theSame || event.EventType == connector.POD_CREATED && pod.Status == event.Status && pod.Node == event.Node && pod.Node != nil

	if theSame {
		log.Info().Msgf("ignoring the event because it didn't change anything")
		return
	}

	if len(scheduler.expectations) > 0 {
		currentExpectation := scheduler.expectations[0]
		if currentExpectation.doMatch(event) {
			if err := currentExpectation.onOccurred(event); err != nil {
				log.Err(err).Msgf("couldn't execute expectation's onOccurred due to")
				scheduler.flushExpectations(false)
			} else {
				scheduler.nextExpectation()
				return
			}
		} else {

			if !podCreation && !podStatusChange {
				log.Warn().Msgf("got an event that does not match expectation")
				scheduler.flushExpectations(false)
			}
		}
	}

	log.Info().Msgf("here with %v", event.EventType)

	switch event.EventType {
	case connector.POD_CREATED:
		// The pod is created.
		scheduler.newPodBuffer = append(scheduler.newPodBuffer, pod)

	case connector.POD_CHANGED:
		// Sync pod nodes
		if pod.Node != event.Node {
			podNodeId := -1
			if pod.Node != nil {
				podNodeId = pod.Node.Id
			}

			log.Error().Msgf(
				"pod changed to a unwanted node, wanted %d, got %d",
				podNodeId,
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

			scheduler.flushExpectations(true)
		}

		// Sync pod states.
		if pod.Status != event.Status {
			if pod.Status == model.RUNNING {
				scheduler.clusterState.NumberOfRunningPods[pod.Deployment.Id] -= 1
			} else if event.Status == model.RUNNING {
				scheduler.clusterState.NumberOfRunningPods[pod.Deployment.Id] += 1
			}
			pod.Status = event.Status

			if pod.Status == model.FINISHED {
				log.Info().Msgf("pod %d has been finished", pod.Id)

				// Remove it from edge, because it don't get resources anymore.
				// if pod.Node != nil {
				// 	scheduler.clusterState.RemovePod(pod)
				// }
			}
		}

	case connector.POD_DELETED:
		log.Info().Msgf("pod %d has been deleted", pod.Id)
		scheduler.clusterState.RemovePod(pod)
		scheduler.flushExpectations(true)
	}
}

func (scheduler *Scheduler) schedulePlan(plan []*planElement) {
	log.Info().Msg("scheduling plan")
	if len(plan) == 0 {
		log.Info().Msg("plan was empty")
		return
	}

	log.Info().Msg("planning")
	if err := plan[0].do(nil); err != nil {
		log.Err(err).Send()
		scheduler.flushExpectations(false)
	}

	for i := 0; i < len(plan)-1; i++ {
		log.Info().Msgf("%d %d\n", i+1, len(plan))

		currentPlan := plan[i]
		nextPlan := plan[i+1]

		scheduler.expectations = append(scheduler.expectations, &expectation{
			doMatch: currentPlan.isValid,
			onOccurred: func(event *connector.Event) error {
				log.Info().Msg("on next after")
				if err := currentPlan.after(event); err != nil {
					return err
				}
				log.Info().Msg("on next plan")
				if err := nextPlan.do(event); err != nil {
					return err
				}

				return nil
			},
			id: uuid.New().ID(),
		})
	}

	lastPlan := plan[len(plan)-1]
	scheduler.expectations = append(scheduler.expectations, &expectation{
		doMatch: lastPlan.isValid,
		onOccurred: func(event *connector.Event) error {
			if err := lastPlan.after(event); err != nil {
				return err
			}

			return nil
		},
		id: uuid.New().ID(),
	})
}

func (scheduler *Scheduler) schedule() {
	log.Info().Msg("scheduling requested")
	if len(scheduler.expectations) != 0 {
		log.Info().Msgf("still expect something, checking new pods\n")

		progress := true
		for len(scheduler.expectations) > 0 && progress {
			progress = false

			currentExpectation := scheduler.expectations[0]
			for ind, pod := range scheduler.newPodBuffer {
				fakeEvent := &connector.Event{
					EventType: connector.POD_CREATED,
					Pod:       pod,
					Node:      nil,
					Status:    pod.Status,
				}
				if currentExpectation.doMatch(fakeEvent) {
					if err := currentExpectation.onOccurred(fakeEvent); err != nil {
						log.Err(err).Msgf("couldn't execute expectation's onOccurred due to")
						scheduler.flushExpectations(false)
						break
					} else {
						scheduler.nextExpectation()
						progress = true

						scheduler.newPodBuffer[ind] = scheduler.newPodBuffer[len(scheduler.newPodBuffer)-1]
						scheduler.newPodBuffer = scheduler.newPodBuffer[:len(scheduler.newPodBuffer)-1]

						break
					}
				}
			}
		}

		return
	}

	newPodsLength := utils.Min(len(scheduler.newPodBuffer), config.SchedulerGeneralConfig.BatchSize)
	newPods := scheduler.newPodBuffer[:newPodsLength]
	scheduler.newPodBuffer = scheduler.newPodBuffer[newPodsLength:]

	decision := alg.MakeDecisionForNewPods(scheduler.clusterState, newPods)

	log.Info().Msgf("decision has been made %v", decision)

	cloudNode := scheduler.clusterState.Cloud.Nodes[0]

	var plan []*planElement
	imgState := scheduler.clusterState.Clone()
	getImgPod := func(pod *model.Pod) *model.Pod {
		imgPod, ok := imgState.PodsMap[pod.Id]
		if !ok {
			return &model.Pod{
				Id:         pod.Id,
				Deployment: pod.Deployment,
				Node:       pod.Node,
				Status:     pod.Status,
			}
		}
		return imgPod
	}

	for _, pod := range decision.EdgeToCloudOffloadingPods {
		plan = append(plan, getDeletePodPlanElement(scheduler, pod))
		plan = append(plan, getCreatePodPlanElement(scheduler, pod.Deployment))
		plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, cloudNode))

		imgPod := getImgPod(pod)
		imgState.RemovePod(imgPod)
		imgState.DeployCloud(imgPod)
	}

	for _, pod := range decision.ToCloudPods {
		plan = append(plan, getBindPodPlanElement(scheduler, pod, cloudNode))

		imgPod := getImgPod(pod)
		imgState.DeployCloud(imgPod)
	}

	for _, migration := range decision.Migrations {
		pod := migration.Pod
		node := migration.Node

		imgPod := getImgPod(pod)
		imgState.RemovePod(imgPod)

		plan = append(plan, getDeletePodPlanElement(scheduler, pod))
		plan = append(plan, getCreatePodPlanElement(scheduler, pod.Deployment))

		if err := imgState.DeployEdge(imgPod, node); err == nil {
			plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, node))
		} else {
			imgState.DeployCloud(imgPod)
			plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, cloudNode))
		}
	}

	edgeMapping := alg.MapPodToEdge(scheduler.clusterState, decision.ToEdgePods, decision.EdgeToCloudOffloadingPods, decision.Migrations).Mapping

	for _, pod := range decision.ToEdgePods {
		if node, ok := edgeMapping[pod.Id]; ok {
			plan = append(plan, getBindPodPlanElement(scheduler, pod, node))
		} else {
			log.Warn().Msgf("couldn't deploy pod %d on edge, deploying on cloud", pod.Id)
			plan = append(plan, getBindPodPlanElement(scheduler, pod, cloudNode))
		}
	}

	scheduler.schedulePlan(plan)
}

func (scheduler *Scheduler) checkSuggestion(suggestion model.CloudSuggestion) {
	log.Info().Msg("checking suggestion")
	if len(scheduler.expectations) != 0 {
		return
	}

	cloudNode := scheduler.clusterState.Cloud.Nodes[0]

	newPods := make([]*model.Pod, 0)

	for _, suggestionPod := range suggestion.Migrations {
		pod, ok := scheduler.clusterState.PodsMap[suggestionPod.Id]
		if !ok {
			continue
		}

		if pod.Node != cloudNode {
			continue
		}

		newPods = append(newPods, pod)
	}

	var plan []*planElement
	for _, pod := range newPods {
		plan = append(plan, getDeletePodPlanElement(scheduler, pod))
	}

	scheduler.schedulePlan(plan)
}

func (scheduler *Scheduler) resetClusterView() error {
	pendingPods, err := scheduler.connector.SyncPods()
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("couldn't re-sync pods")
	}

	scheduler.newPodBuffer = nil
	scheduler.newPodBuffer = append(scheduler.newPodBuffer, pendingPods...)

	return nil
}

func (scheduler *Scheduler) recoverHealth() {
	log.Warn().Msg("resetting scheduler's view of cluster...")

	recoverRetryDuration := time.Duration(config.SchedulerGeneralConfig.RecoverRetryDuration) * time.Millisecond

	err := scheduler.resetClusterView()
	if err != nil {
		log.Err(err).Msg("couldn't reset due to the error")
		log.Warn().Msg("waiting for couple of seconds and retrying")
		time.Sleep(recoverRetryDuration)
		scheduler.recoverHealth()
	}

	scheduler.healthCheckSample = nil
	scheduler.flushExpectations(false)

	log.Info().Msg("done with resetting scheduler's view of cluster")
}

func (scheduler *Scheduler) checkHealth() {
	log.Info().Msg("checking scheduler's health...")

	newSample := newHealthCheckSample(scheduler)
	if !newSample.isTheSame(scheduler.healthCheckSample) {
		log.Info().Msg("health check done, everything looks fine")
		scheduler.healthCheckSample = newSample
		return
	}

	log.Warn().Msg("the scheduler is not healthy :(")
	scheduler.recoverHealth()
	log.Info().Msg("the scheduler health recovered, continuing...")
}

func (scheduler *Scheduler) Run(ctx context.Context) (SchedulerBridge, error) {
	log.Info().Msg("scheduler is running...")

	eventStream, err := scheduler.connector.WatchSchedulingEvents()
	if err != nil {
		log.Err(err).Send()

		return SchedulerBridge{}, fmt.Errorf("could not start watching scheduling events")
	}
	log.Info().Msg("got event watcher from connector")

	scheduleTicker := time.NewTicker(time.Duration(config.SchedulerGeneralConfig.FlushPeriodDuration) * time.Millisecond)
	healthCheckTicker := time.NewTicker(time.Duration(config.SchedulerGeneralConfig.HealthCheckDuration) * time.Millisecond)
	cloudSuggestionDuration := time.Duration(config.SchedulerGeneralConfig.CloudSuggestDuration) * time.Millisecond

	clusterStateRequestStream := make(chan struct{})
	clusterStateStream := make(chan *model.ClusterState, 1024)

	makeCloudSuggestion := make(chan struct{})

	CloudSuggestStream := make(chan model.CloudSuggestion)
	go func() {
		<-time.After(cloudSuggestionDuration)
		makeCloudSuggestion <- struct{}{}
	}()

	go func() {
	scheduler_live:
		for {
			select {
			case <-ctx.Done():
				scheduleTicker.Stop()
				break scheduler_live
			case event := <-eventStream:
				scheduler.handleEvent(event)
			case <-scheduleTicker.C:
				scheduler.schedule()
			case <-healthCheckTicker.C:
				scheduler.checkHealth()
			case <-clusterStateRequestStream:
				clusterStateStream <- scheduler.clusterState.Clone()
			case <-makeCloudSuggestion:
				clonedState := scheduler.clusterState.Clone()
				go func() {
					log.Info().Msg("making suggestion")
					CloudSuggestStream <- alg.SuggestCloudToEdge(clonedState)
					<-time.After(cloudSuggestionDuration)
					makeCloudSuggestion <- struct{}{}
				}()
			case suggestion := <-CloudSuggestStream:
				scheduler.checkSuggestion(suggestion)
			}
		}
	}()
	log.Info().Msg("set up scheduler's main life cycle")

	return SchedulerBridge{
		ClusterStateRequestStream: clusterStateRequestStream,
		ClusterStateStream:        clusterStateStream,
	}, nil
}
