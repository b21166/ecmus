package connector

import (
	"context"
	"fmt"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"github.com/rs/zerolog/log"
	"gonum.org/v1/gonum/mat"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeConnector struct {
	clientset    *kubernetes.Clientset
	clusterState *model.ClusterState

	nodeIdToName       map[int]string
	deploymentIdToName map[int]string
}

func NewKubeConnector(configPath string) (*KubeConnector, error) {
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
		clientset:    clientset,
		clusterState: model.NewClusterState(),
		nodeIdToName: make(map[int]string),
	}

	if err := kc.FindNodes(); err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not find nodes")
	}

	if err := kc.FindDeployments(); err != nil {
		log.Err(err).Send()

		return nil, fmt.Errorf("could not find deployments")
	}

	return kc, nil
}

func (kc *KubeConnector) FindNodes() error {
	ctx := context.Background()
	nodeList, err := kc.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not list nodes")
	}

	for _, node := range nodeList.Items {
		modelNode := &model.Node{
			Id: utils.Hash(node.GetObjectMeta().GetName()),
			Resources: mat.NewVecDense(2, []float64{
				node.Status.Allocatable.Cpu().AsApproximateFloat64(),
				node.Status.Allocatable.Memory().AsApproximateFloat64(),
			}),
		}

		clusterType, ok := node.GetObjectMeta().GetLabels()[config.SchedulerGeneralConfig.Name+"/cluster-type"]
		if !ok || clusterType == "ignore" {
			continue
		}

		kc.clusterState.AddNode(modelNode, clusterType)
	}

	podList, err := kc.clientset.CoreV1().Pods(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not get pods list")
	}

	// remove existing pods from node resources
	for _, pod := range podList.Items {
		// TODO check is not running pods does not allocate memory
		if pod.Spec.NodeName == "" || pod.Status.Phase != v1.PodRunning {
			continue
		}

		nodeId := utils.Hash(pod.Spec.NodeName)

		for _, node := range kc.clusterState.Edge.Config.Nodes {
			if node.Id != nodeId {
				continue
			}
			for _, container := range pod.Spec.Containers {
				vec := mat.NewVecDense(2, []float64{
					container.Resources.Limits.Cpu().AsApproximateFloat64(),
					container.Resources.Limits.Memory().AsApproximateFloat64(),
				})

				utils.SSubVec(node.Resources, vec)
				utils.SSubVec(kc.clusterState.Edge.Config.Resources, vec)
			}
		}
	}

	return nil
}

func (kc *KubeConnector) FindDeployments() error {
	ctx := context.Background()
	deploymentList, err := kc.clientset.AppsV1().Deployments(config.SchedulerGeneralConfig.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Send()

		return fmt.Errorf("could not list deployments")
	}

	for _, deployment := range deploymentList.Items {
		// TODO score rule

		resourceList := deployment.Spec.Template.Spec.Containers[0].Resources.Limits

		modelDeployment := &model.Deployment{
			Id: utils.Hash(deployment.GetObjectMeta().GetName()),
			ResourcesRequired: mat.NewVecDense(2, []float64{
				resourceList.Cpu().AsApproximateFloat64(),
				resourceList.Memory().AsApproximateFloat64(),
			}),
			ImageSize: 0, // FIXME add real image size
		}

		kc.clusterState.Edge.Config.Deployments = append(kc.clusterState.Edge.Config.Deployments, modelDeployment)
		kc.deploymentIdToName[modelDeployment.Id] = deployment.GetObjectMeta().GetName()
	}

	return nil
}

func (kc *KubeConnector) GetClusterState() *model.ClusterState {
	return kc.clusterState
}
