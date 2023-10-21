package connector

import (
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/model/testing_tool"
)

// This connector in implemented for
// testing purposes and shows a
// "constant" functionality.
type ConstantConnector struct {
	clusterState     *model.ClusterState
	goalClusterState *model.ClusterState
}

func NewConstantConnector(clusterState *model.ClusterState) *ConstantConnector {
	builder := testing_tool.New()
	builder.ImportDeployments([]*testing_tool.DeploymentDesc{
		{Name: "A", Cpu: 1, Memory: 2, EdgeShare: 0.5},
		{Name: "B", Cpu: 1, Memory: 1, EdgeShare: 0.5},
		{Name: "C", Cpu: 0.5, Memory: 1, EdgeShare: 1},
		{Name: "D", Cpu: 2, Memory: 4, EdgeShare: 1},
	})

	goalClusterState := builder.GetCluster(
		map[*testing_tool.NodeDesc][]string{
			{Cpu: 2, Memory: 4}: {"A", "A"},
			{Cpu: 2, Memory: 2}: {},
			{Cpu: 2, Memory: 3}: {},
		},
		[]string{},
	)

	for _, deployment := range goalClusterState.Edge.Config.Deployments {
		clusterState.Edge.Config.AddDeployment(deployment)
	}

	for _, node := range goalClusterState.Edge.Config.Nodes {
		clusterState.AddNode(node, "edge")
	}

	for _, node := range goalClusterState.Cloud.Nodes {
		clusterState.AddNode(node, "cloud")
	}

	for _, pod := range goalClusterState.Edge.Pods {
		clusterState.DeployEdge(pod, pod.Node)
	}

	for _, pod := range goalClusterState.Cloud.Pods {
		clusterState.DeployCloud(pod)
	}

	return &ConstantConnector{
		clusterState:     clusterState,
		goalClusterState: goalClusterState,
	}
}

func (c *ConstantConnector) FindNodes() error {
	return nil
}

func (c *ConstantConnector) SyncPods() ([]*model.Pod, error) {
	return nil, nil
}

func (c *ConstantConnector) FindDeployments() error {
	return nil
}

func (c *ConstantConnector) GetClusterState() *model.ClusterState {
	return c.clusterState
}

func (c *ConstantConnector) Deploy(pod *model.Pod, node *model.Node) error {
	return nil
}

func (c *ConstantConnector) DeletePod(pod *model.Pod) (bool, error) {
	return true, nil
}

func (c *ConstantConnector) WatchSchedulingEvents() (<-chan *Event, error) {
	return make(chan *Event), nil
}
