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
	"k8s.io/client-go/tools/clientcmd"
)

type KubeConnector struct {
	clientset    *kubernetes.Clientset
	clusterState *model.ClusterState

	nodeIdToName       map[int]string
	podIdToName        map[int]string
	deploymentIdToName map[int]string
}

func NewKubeConnector(configPath string, clusterState *model.ClusterState) (*KubeConnector, error) {
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not init kube connector from config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not init clients")
	}

	kc := &KubeConnector{
		clientset:          clientset,
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
				node.Status.Allocatable.Cpu().AsApproximateFloat64() - 1,
				node.Status.Allocatable.Memory().AsApproximateFloat64()/config.MB - 1000,
			}),
		}

		clusterType, ok := node.GetObjectMeta().GetLabels()[config.SchedulerGeneralConfig.Name+"/cluster-type"]
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

func (kc *KubeConnector) SyncPods() error {
	ctx := context.Background()
	podList, err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not get pods list")
	}

	allPods := make([]*model.Pod, len(kc.clusterState.PodsMap))
	for _, pod := range kc.clusterState.PodsMap {
		allPods = append(allPods, pod)
	}

	for _, pod := range allPods {
		if pod != nil {
			kc.clusterState.RemovePod(pod)
		}
	}

	// remove existing pods from node resources
	for _, pod := range podList.Items {
		// TODO check running pods do not allocate memory
		if pod.Spec.NodeName == "" || pod.Status.Phase != v1.PodRunning {
			continue
		}

		deploymentName, ok := pod.ObjectMeta.Labels["app"]
		if !ok {
			continue
		}

		id := utils.Hash(pod.Name)
		nodeId := utils.Hash(pod.Spec.NodeName)
		deploymentId := utils.Hash(deploymentName)

		for _, node := range kc.clusterState.Edge.Config.Nodes {
			if node.Id != nodeId {
				continue
			}
			kc.clusterState.DeployEdge(&model.Pod{
				Id:         id,
				Deployment: kc.clusterState.Edge.Config.DeploymentIdToDeployment[deploymentId],
				Node:       nil,
				Status:     model.RUNNING,
			}, node)
		}

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
		}

		kc.podIdToName[id] = pod.Name
	}

	return nil
}

func (kc *KubeConnector) FindDeployments() error {
	log.Info().Msg("finding deployments...")

	ctx := context.Background()
	deploymentList, err := kc.clientset.AppsV1().Deployments(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not list deployments")
	}

	for _, deployment := range deploymentList.Items {
		resourceList := deployment.Spec.Template.Spec.Containers[0].Resources.Limits

		deploymentName := deployment.GetObjectMeta().GetLabels()["app"]
		modelDeployment := &model.Deployment{
			Id: utils.Hash(deploymentName),
			ResourcesRequired: mat.NewVecDense(2, []float64{
				resourceList.Cpu().AsApproximateFloat64(),
				resourceList.Memory().AsApproximateFloat64() / config.MB,
			}),
			EdgeShare: 0.5, // TODO parse it
		}
		if deploymentName == "c" {
			modelDeployment.EdgeShare = 1
		}

		log.Info().Msgf("found deployment %s", deploymentName)
		kc.clusterState.Edge.Config.AddDeployment(modelDeployment)
		kc.deploymentIdToName[modelDeployment.Id] = deploymentName
	}

	log.Info().Msg("deployments found")

	kc.SyncPods()

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

	err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).Delete(
		context.Background(), podName, *metav1.NewDeleteOptions(0),
	)
	if err != nil {
		return true, err
	}

	return true, nil
}

func (kc *KubeConnector) Deploy(pod *model.Pod, node *model.Node) error {
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
	go func() {
		for event := range watcher.ResultChan() {
			v1Pod, ok := event.Object.(*v1.Pod)
			if !ok {
				// the event is not about a pod
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

			switch v1Pod.Status.Phase {
			case v1.PodPending, v1.PodUnknown:
				newPodStatus = model.SCHEDULED
			case v1.PodRunning:
				newPodStatus = model.RUNNING
			case v1.PodSucceeded, v1.PodFailed:
				newPodStatus = model.FINISHED
			}

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
