// TODO decouple connector from cluster state

package connector

import (
	"context"
	"fmt"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"gonum.org/v1/gonum/mat"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubeConnector struct {
	// Kubernetes official library client for
	// contacting API-server.
	clientset *kubernetes.Clientset

	// The shared cluster state.
	clusterState *model.ClusterState

	// Mappings for getting pod, node,
	// and deployments names easily.
	nodeIdToName       map[int]string
	podIdToName        map[int]string
	deploymentIdToName map[int]string
}

func NewKubeConnector(clusterState *model.ClusterState) (*KubeConnector, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("can't connect to kubernetes cluster")
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not init clients")
	}

	kc := &KubeConnector{
		clientset:          clientSet,
		clusterState:       clusterState,
		nodeIdToName:       make(map[int]string),
		podIdToName:        make(map[int]string),
		deploymentIdToName: make(map[int]string),
	}

	return kc, nil
}

func (kc *KubeConnector) FindNodes() error {
	log.Info().Msg("finding nodes...")

	ctx := context.Background()
	// the node list from kubernetes
	nodeList, err := kc.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not list nodes")
	}

	for _, node := range nodeList.Items {
		nodeName := node.GetObjectMeta().GetName()

		modelNode := &model.Node{
			Id: utils.Hash(nodeName),
			Resources: mat.NewVecDense(2, []float64{
				// Removing 1 core and 1 Gig from CPU and memory
				// of each node so background processes and not visible
				// pods to scheduler won't cause "Out Of Resource" error
				// during scheduler execution.
				// TODO Scheduler should be robust to OOR errors.
				// FIXME Scheduler can approximate nodes used resources
				// FIXME in better ways like htop or trial and error.
				node.Status.Allocatable.Cpu().AsApproximateFloat64() - 1,
				node.Status.Allocatable.Memory().AsApproximateFloat64()/config.MB - 1000,
			}),
		}

		// "nodetype" label categorize that the node is either
		// cloud, edge or non.
		// This label should be in object/meta.
		clusterType, ok := node.GetObjectMeta().GetLabels()["nodetype"]
		if !ok || clusterType == "ignore" {
			continue
		}

		log.Info().Msgf("found node %s", node.GetObjectMeta().GetName())
		kc.clusterState.AddNode(modelNode, clusterType)
		kc.nodeIdToName[modelNode.Id] = nodeName
	}

	log.Info().Msg("nodes found")

	return nil
}

func (kc *KubeConnector) GetPendingPods() ([]*model.Pod, error) {
	pendingPods := make([]*model.Pod, 0)

	// getting the pod list from scheduler's namespace.
	ctx := context.Background()
	podList, err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not get pods list")
	}

	// checking all pods for pending pods
	for _, pod := range podList.Items {
		deploymentName, ok := pod.ObjectMeta.Labels["app"]
		if !ok {
			continue
		}

		id := utils.Hash(pod.Name)
		deploymentId := utils.Hash(deploymentName)

		deployment, ok := kc.clusterState.Edge.Config.DeploymentIdToDeployment[deploymentId]
		if !ok {
			continue
		}

		if pod.Status.Phase == v1.PodPending && pod.Spec.NodeName == "" {
			pendingPods = append(pendingPods, &model.Pod{
				Id:         id,
				Deployment: deployment,
				Node:       nil,
				Status:     model.SCHEDULED,
			})
		}
	}

	return pendingPods, nil
}

func (kc *KubeConnector) SyncPods() ([]*model.Pod, error) {
	// getting all pods that exists in the cluster state
	allPods := make([]*model.Pod, len(kc.clusterState.PodsMap))
	for _, pod := range kc.clusterState.PodsMap {
		allPods = append(allPods, pod)
	}

	// erasing all of them from cluster state
	for _, pod := range allPods {
		if pod != nil {
			kc.clusterState.RemovePod(pod)
		}
	}

	pendingPods := make([]*model.Pod, 0)

	// getting the pod list from scheduler's namespace.
	ctx := context.Background()
	// TODO check running other pods in other namespaces do not allocate memory
	podList, err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not get pods list")
	}

	// checking each pod and deploying it where it belongs to.
	for _, pod := range podList.Items {
		// TODO check other running pods do not allocate memory
		deploymentName, ok := pod.ObjectMeta.Labels["app"]
		if !ok {
			continue
		}

		id := utils.Hash(pod.Name)
		deploymentId := utils.Hash(deploymentName)

		deployment, ok := kc.clusterState.Edge.Config.DeploymentIdToDeployment[deploymentId]
		if !ok {
			continue
		}

		isRunning := (pod.Status.Phase == v1.PodPending && pod.Spec.NodeName != "")
		isRunning = (isRunning || pod.Status.Phase == v1.PodRunning)

		if pod.Status.Phase == v1.PodPending && pod.Spec.NodeName == "" {
			pendingPods = append(pendingPods, &model.Pod{
				Id:         id,
				Deployment: deployment,
				Node:       nil,
				Status:     model.SCHEDULED,
			})

			continue
		}

		// TODO check running pods do not allocate memory
		if !isRunning {
			log.Info().Msgf("ignoring a pod named %s due to its status", pod.Name)
			continue
		}

		nodeId := utils.Hash(pod.Spec.NodeName)
		foundNode := false

		// checks whether it is on edge or not
		for _, node := range kc.clusterState.Edge.Config.Nodes {
			if node.Id != nodeId {
				continue
			}

			kc.clusterState.DeployEdge(&model.Pod{
				Id:         id,
				Deployment: deployment,
				Node:       nil,
				Status:     model.RUNNING,
			}, node)
			kc.clusterState.NumberOfRunningPods[deploymentId] += 1

			if foundNode {
				panic(fmt.Errorf("multiple nodes with the same id as %d", nodeId))
			}
			foundNode = true
		}

		// checks whether it is on cloud or not
		for _, node := range kc.clusterState.Cloud.Nodes {
			if node.Id != nodeId {
				continue
			}
			kc.clusterState.DeployCloud(&model.Pod{
				Id:         id,
				Deployment: kc.clusterState.Edge.Config.DeploymentIdToDeployment[deploymentId],
				Node:       nil,
				Status:     model.RUNNING,
			})
			kc.clusterState.NumberOfRunningPods[deploymentId] += 1

			if foundNode {
				panic(fmt.Errorf("multiple nodes with the same id as %d", nodeId))
			}
			foundNode = true
		}

		if !foundNode {
			// should not happen
			log.Error().Msgf("found a pod named %s on node %s that the node is on neither cloud nor edge", pod.Name, pod.Spec.NodeName)
			continue
		}

		kc.podIdToName[id] = pod.Name
	}

	return pendingPods, nil
}

func (kc *KubeConnector) FindDeployments() error {
	log.Info().Msg("finding deployments...")

	ctx := context.Background()
	// the list of deployments in the namespace
	deploymentList, err := kc.clientset.AppsV1().Deployments(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not list deployments")
	}

	for _, deployment := range deploymentList.Items {
		resourceList := deployment.Spec.Template.Spec.Containers[0].Resources.Limits

		// deployment name should be stored in a label in object/meta with key "app"
		deploymentName := deployment.GetObjectMeta().GetLabels()["app"]
		schedulerNameLen := len(config.SchedulerGeneralConfig.Name)
		if len(deploymentName) >= schedulerNameLen && deploymentName[:schedulerNameLen] == config.SchedulerGeneralConfig.Name {
			// ignore your pod
			// WARN scheduler should not be a node which scheduler can scheduler a pod on!
			continue
		}

		modelDeployment := &model.Deployment{
			Id: utils.Hash(deploymentName),
			ResourcesRequired: mat.NewVecDense(2, []float64{
				resourceList.Cpu().AsApproximateFloat64(),
				resourceList.Memory().AsApproximateFloat64() / config.MB,
			}),
			EdgeShare: 1, // TODO parse it from deployment's labels
		}
		// FIXME manual edge share:
		// if strings.Contains(strings.ToLower(deploymentName), "d") {
		// 	modelDeployment.EdgeShare = 0.5
		// }

		log.Info().Msgf("found deployment %s", deploymentName)
		kc.clusterState.Edge.Config.AddDeployment(modelDeployment)
		kc.deploymentIdToName[modelDeployment.Id] = deploymentName
	}

	log.Info().Msg("deployments found")

	// After finding deployments, it's time for searching for pods.
	if _, err := kc.SyncPods(); err != nil {
		log.Err(err).Send()

		return fmt.Errorf("couldn't sync pods")
	}

	return nil
}

func (kc *KubeConnector) GetClusterState() *model.ClusterState {
	return kc.clusterState
}

func (kc *KubeConnector) DeletePod(pod *model.Pod) (bool, error) {
	podName, ok := kc.podIdToName[pod.Id]
	if !ok {
		return false, nil
	}
	delete(kc.podIdToName, pod.Id)

	// k8s delete API
	err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).Delete(
		context.Background(), podName, *metav1.NewDeleteOptions(0),
	)
	if err != nil {
		return true, err
	}

	return true, nil
}

func (kc *KubeConnector) Deploy(pod *model.Pod, node *model.Node) error {
	if node == nil {
		return fmt.Errorf("cannot deploy a pod on a nil node")
	}

	log.Info().Msgf(
		"deploying pod %d to node %d",
		pod.Id,
		node.Id,
	)

	if node == nil {
		return fmt.Errorf("the pod is not allocated to any node")
	}

	nodeName, ok := kc.nodeIdToName[node.Id]
	if !ok {
		return fmt.Errorf("the pod's node is not mapped to a known node")
	}

	podName, ok := kc.podIdToName[pod.Id]
	if !ok {
		return fmt.Errorf("the pod is not known")
	}

	target := v1.ObjectReference{
		Kind:       "Node",
		APIVersion: "v1",
		Name:       nodeName,
	}

	objectMeta := metav1.ObjectMeta{
		Name:      podName,
		Namespace: config.SchedulerGeneralConfig.Namespace,
	}

	binding := &v1.Binding{
		ObjectMeta: objectMeta,
		Target:     target,
	}

	// A k8s binding is created for deploying a pod on a node.
	err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).Bind(
		context.Background(),
		binding,
		metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (kc *KubeConnector) WatchSchedulingEvents() (<-chan *Event, error) {
	// k8s API for watching events of a namespace:
	watcher, err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).Watch(
		context.Background(),
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.schedulerName=%s", config.SchedulerGeneralConfig.Name),
		},
	)
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not start watching cluster events")
	}

	eventStream := make(chan *Event)
	// The goroutine duty is to translate all k8s events
	// to an internal event and send it through eventStream.
	go func() {
		for event := range watcher.ResultChan() {
			v1Pod, ok := event.Object.(*v1.Pod)
			if !ok {
				// the event is not about a pod
				// TODO maybe check for
				continue
			}

			deploymentName, ok := v1Pod.ObjectMeta.Labels["app"]
			if !ok {
				log.Warn().Msgf("event's pod has no deployment.")

				continue
			}

			deploymentId := utils.Hash(deploymentName)
			deployment, ok := kc.clusterState.Edge.Config.DeploymentIdToDeployment[deploymentId]
			if !ok {
				log.Info().Msgf("some pod event has happened for deployment %s not related to scheduler.", deploymentName)

				continue
			}

			id := utils.Hash(v1Pod.Name)
			pod, isOldPod := kc.clusterState.PodsMap[id]
			if !isOldPod {
				log.Info().Msgf("got an event about not registered pod, creating it.")
				kc.podIdToName[id] = v1Pod.Name
				pod = &model.Pod{
					Id:         id,
					Deployment: deployment,
				}
				kc.clusterState.PodsMap[pod.Id] = pod
			}

			nodeName := v1Pod.Spec.NodeName
			var node *model.Node
			if nodeName == "" {
				node = nil
			} else {
				nodeId := utils.Hash(nodeName)
				node, ok = kc.clusterState.GetNodeIdToNode()[nodeId]
				if !ok {
					log.Warn().Msgf("pod's node (%s) is not registered, ignoring the event.", nodeName)

					return
				}
			}

			var newPodStatus model.PodStatus
			log.Info().Msgf("pod's kubernetes status: %s", v1Pod.Status.Phase)

			// translating the pod status:
			switch v1Pod.Status.Phase {
			case v1.PodPending, v1.PodUnknown:
				newPodStatus = model.SCHEDULED
			case v1.PodRunning:
				newPodStatus = model.RUNNING
			case v1.PodSucceeded, v1.PodFailed:
				newPodStatus = model.FINISHED
			}

			// inferring event type:
			var eventType EventType
			switch event.Type {
			case watch.Added:
				eventType = POD_CREATED
			case watch.Modified:
				eventType = POD_CHANGED
			case watch.Deleted:
				eventType = POD_DELETED
			}

			if eventType == POD_CREATED && isOldPod {
				continue
			}

			eventStream <- &Event{
				EventType: eventType,
				Pod:       pod,
				Node:      node,
				Status:    newPodStatus,
			}
		}
	}()

	return eventStream, nil
}
