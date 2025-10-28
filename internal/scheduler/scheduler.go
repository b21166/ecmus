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
	"github.com/amsen20/ecmus/statistics"
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

	goingToPlace map[int]bool
	newPodBuffer []*model.Pod

	// Expectations must be either all PLACING or REORDERING not a mixture of them.
	// TODO refactor this to a interface with two implementations PLACING and REORDERING
	expectations               []*expectation
	expectedReorderDeployments map[int]int

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

		goingToPlace:               make(map[int]bool),
		expectedReorderDeployments: make(map[int]int),
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

	scheduler.goingToPlace = make(map[int]bool)
	scheduler.expectedReorderDeployments = make(map[int]int)

	scheduler.expectations = nil
	if reschedule {
		scheduler.schedule()
	}
}

func (scheduler *Scheduler) nextExpectation() {
	if len(scheduler.expectations) > 0 {
		statistics.Change(
			fmt.Sprintf(
				"expectation type %s done", expectationTypeToString(
					scheduler.expectations[0].tp,
				)), 1,
		)

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

func (scheduler *Scheduler) schedulePlan(plan []*planElement, planType expectationType) {
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
			tp: planType,
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
		tp: planType,
	})

	statistics.Change(fmt.Sprintf("expectation type %s added", expectationTypeToString(planType)), len(plan))
}

func (scheduler *Scheduler) schedule() {
	log.Info().Msg("scheduling requested")
	if len(scheduler.expectations) > 0 && scheduler.expectations[0].tp == PLACING {
		log.Info().Msg("ignored scheduling because last placing is not done yet")
		return
	}

	if len(scheduler.newPodBuffer) == 0 {
		// An optimization:
		// Fetch pending pods from k8s api if the buffer is empty.

		pendingPods, err := scheduler.connector.GetPendingPods()
		if err == nil {
			scheduler.newPodBuffer = pendingPods
		}
	}

	if len(scheduler.expectations) > 0 {
		progress := true
		for len(scheduler.expectations) > 0 && progress {
			progress = false

			currentExpectation := scheduler.expectations[0]
			for ind, pod := range scheduler.newPodBuffer {
				dummyEvent := &connector.Event{
					EventType: connector.POD_CREATED,
					Pod:       pod,
					Node:      nil,
					Status:    pod.Status,
				}
				if currentExpectation.doMatch(dummyEvent) {
					if err := currentExpectation.onOccurred(dummyEvent); err != nil {
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
	}

	var filteredBuffer []*model.Pod
	for _, newPod := range scheduler.newPodBuffer {
		if _, ok := scheduler.goingToPlace[newPod.Id]; !ok {
			filteredBuffer = append(filteredBuffer, newPod)
		}
	}
	scheduler.newPodBuffer = filteredBuffer

	if len(scheduler.newPodBuffer) == 0 {
		log.Info().Msg("no *new* pod for scheduling")
		return
	}

	// This code is responsible to check whether the pending pods
	// are a part of a migration or are new pods?
	if len(scheduler.expectations) > 0 {
		deploymentCounts := make(map[int]int)
		for _, newPod := range scheduler.newPodBuffer {
			cnt := deploymentCounts[newPod.Deployment.Id]
			deploymentCounts[newPod.Deployment.Id] = cnt + 1
		}

		for deploymentId, cnt := range deploymentCounts {
			if cnt > scheduler.expectedReorderDeployments[deploymentId] {
				scheduler.flushExpectations(true)
				return
			}
		}

		scheduler.newPodBuffer = nil
	}

	if len(scheduler.newPodBuffer) == 0 {
		log.Info().Msg("all current pending pods are a part of a migration")
		return
	}

	if len(scheduler.expectations) > 0 {
		log.Info().Msg("new pods are arrived in middle of migrations (reordering)")
		scheduler.flushExpectations(false)
	}

	newPodsLength := utils.Min(len(scheduler.newPodBuffer), config.SchedulerGeneralConfig.BatchSize)
	newPods := scheduler.newPodBuffer[:newPodsLength]
	scheduler.newPodBuffer = scheduler.newPodBuffer[newPodsLength:]

	decision := alg.MakeDecisionForNewPods(scheduler.clusterState, newPods, false)

	log.Info().Msgf("decision has been made %v", decision)

	cloudNode := scheduler.clusterState.Cloud.Nodes[0]

	var plan []*planElement
	for _, pod := range decision.ToCloudPods {
		plan = append(plan, getBindPodPlanElement(scheduler, pod, cloudNode))

		scheduler.goingToPlace[pod.Id] = true
	}

	edgeMapping := alg.MapPodToEdge(scheduler.clusterState, decision.ToEdgePods, decision.EdgeToCloudOffloadingPods, decision.Migrations).Mapping

	for _, pod := range decision.ToEdgePods {
		if node, ok := edgeMapping[pod.Id]; ok {
			plan = append(plan, getBindPodPlanElement(scheduler, pod, node))
		} else {
			log.Warn().Msgf("couldn't deploy pod %d on edge, deploying on cloud", pod.Id)
			plan = append(plan, getBindPodPlanElement(scheduler, pod, cloudNode))
		}

		scheduler.goingToPlace[pod.Id] = true
	}

	scheduler.schedulePlan(plan, PLACING)
}

func (scheduler *Scheduler) checkSuggestion(suggestion model.ReorderSuggestion) {
	log.Info().Msg("checking suggestion")
	if len(scheduler.expectations) != 0 {
		log.Info().Msg("scheduler is in middle of something, ignored the suggestion")
		return
	}

	// Resetting everything.
	scheduler.flushExpectations(false)

	cloudNode := scheduler.clusterState.Cloud.Nodes[0]
	updatedDecision := model.DecisionForNewPods{}
	plan := make([]*planElement, 0)

	canBeFreedFromCloud := make(map[int]*model.Pod)
	for _, suggestionPod := range suggestion.CloudToEdgePods {
		pod, ok := scheduler.clusterState.PodsMap[suggestionPod.Id]
		if !ok {
			continue
		}

		if pod.Node == nil {
			continue
		}

		if pod.Node == cloudNode {
			canBeFreedFromCloud[pod.Id] = pod
		}
	}

	for _, suggestedPod := range suggestion.Decision.ToEdgePods {
		pod, ok := canBeFreedFromCloud[suggestedPod.Id]
		if !ok {
			continue
		}

		updatedDecision.ToEdgePods = append(updatedDecision.ToEdgePods, pod)
	}

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

	for _, suggestionPod := range suggestion.Decision.EdgeToCloudOffloadingPods {
		pod, ok := scheduler.clusterState.PodsMap[suggestionPod.Id]
		if !ok || pod.Node == nil || pod.Node.Id == cloudNode.Id {
			// Either already deleted or placed on cloud.
			continue
		}

		plan = append(plan, getDeletePodPlanElement(scheduler, pod))
		plan = append(plan, getCreatePodPlanElement(scheduler, pod.Deployment))
		plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, cloudNode))

		imgPod := getImgPod(pod)
		imgState.RemovePod(imgPod)
		imgState.DeployCloud(imgPod)

		updatedDecision.EdgeToCloudOffloadingPods = append(
			updatedDecision.EdgeToCloudOffloadingPods,
			pod,
		)
	}

	for _, suggestedMigration := range suggestion.Decision.Migrations {
		pod, ok := scheduler.clusterState.PodsMap[suggestedMigration.Pod.Id]
		node := suggestedMigration.Node

		if !ok || suggestedMigration.Pod.Node.Id != node.Id {
			// Already deleted or changed the node.
			continue
		}

		if node == nil {
			panic("TRIED TO MIGRATE A POD WHICH DOES NOT HAVE NODE")
		}

		imgPod := getImgPod(pod)
		imgState.RemovePod(imgPod)

		if pod.Node != nil {
			// Need to be migrated:
			plan = append(plan, getDeletePodPlanElement(scheduler, pod))
			plan = append(plan, getCreatePodPlanElement(scheduler, pod.Deployment))

			if err := imgState.DeployEdge(imgPod, node); err == nil {
				plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, node))

				updatedDecision.Migrations = append(
					updatedDecision.Migrations,
					&model.Migration{
						Pod:  pod,
						Node: node,
					},
				)
			} else {
				// * Important: the pod is placed on cloud but not on the target of migration
				// * because the next phase (being deleted from cloud and placed on edge) is hoped
				// * to be done by the suggestion system.

				imgState.DeployCloud(imgPod)
				plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, cloudNode))

				updatedDecision.Migrations = append(
					updatedDecision.Migrations, &model.Migration{
						Pod:  pod,
						Node: cloudNode,
					},
				)
			}
		} else {
			// Only need to be placed:
			if err := imgState.DeployEdge(imgPod, node); err == nil {
				plan = append(plan, getBindPodPlanElement(scheduler, pod, node))

				updatedDecision.Migrations = append(
					updatedDecision.Migrations,
					&model.Migration{
						Pod:  pod,
						Node: node,
					},
				)
			} else {
				imgState.DeployCloud(imgPod)
				plan = append(plan, getBindPodPlanElement(scheduler, pod, cloudNode))

				updatedDecision.Migrations = append(
					updatedDecision.Migrations,
					&model.Migration{
						Pod:  pod,
						Node: cloudNode,
					},
				)
			}
		}
	}

	// ToCloudPods are already on cloud,
	// so nothing to do with decision.ToCloudPods.

	edgeMapping := alg.MapPodToEdge(scheduler.clusterState, updatedDecision.ToEdgePods, updatedDecision.EdgeToCloudOffloadingPods, updatedDecision.Migrations).Mapping

	for _, pod := range updatedDecision.ToEdgePods {
		if node, ok := edgeMapping[pod.Id]; ok {
			// Migrate from cloud to edge:
			plan = append(plan, getDeletePodPlanElement(scheduler, pod))
			plan = append(plan, getCreatePodPlanElement(scheduler, pod.Deployment))
			plan = append(plan, getMigrateBindPodPlanElement(scheduler, pod.Deployment, node))
		} else {
			// It is already on cloud, so no need to do anything.
		}

		scheduler.goingToPlace[pod.Id] = true
	}

	// Adding deleted pods to expected reorder deployments:
	for _, pod := range updatedDecision.EdgeToCloudOffloadingPods {
		if pod.Node != nil {
			scheduler.expectedReorderDeployments[pod.Deployment.Id] += 1
		}
	}
	for _, migration := range updatedDecision.Migrations {
		pod := migration.Pod
		if pod.Node != nil {
			scheduler.expectedReorderDeployments[pod.Deployment.Id] += 1
		}
	}

	scheduler.schedulePlan(plan, REORDERING)
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
	statistics.Change("restarts", 1)

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
	if !newSample.isStuck(scheduler.healthCheckSample) {
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

	reorderSuggestStream := make(chan model.ReorderSuggestion)
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
					reorderSuggestStream <- alg.SuggestReorder(clonedState)
					<-time.After(cloudSuggestionDuration)
					makeCloudSuggestion <- struct{}{}
				}()
			case suggestion := <-reorderSuggestStream:
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
