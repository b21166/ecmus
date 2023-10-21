package model

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/utils"
	"github.com/amsen20/ecmus/logging"
	"gonum.org/v1/gonum/mat"
)

var log = logging.Get()

// Stores static properties of the edge cluster
type EdgeConfig struct {
	Nodes                    []*Node
	Deployments              []*Deployment
	DeploymentIdToDeployment map[int]*Deployment

	Resources *mat.VecDense
}

// Stores dynamic properties of the edge
type EdgeState struct {
	Config *EdgeConfig

	Pods          []*Pod
	UsedResources *mat.VecDense
}

// Stores dynamic and static properties of the cloud.
// Scheduler assumption is that all cloud nodes are symmetric
// and so schedulers the pod on them randomly.
type CloudState struct {
	Nodes []*Node
	Pods  []*Pod
}

// The whole state of the cluster in
// scheduler's point of view.
type ClusterState struct {
	Edge  *EdgeState
	Cloud *CloudState

	// Deprecated:
	// CandidatesList      []*Candidate

	// amount of resource used for all nodes
	NodeResourcesUsed map[int]*mat.VecDense
	// a mapping from pod id to pod
	PodsMap map[int]*Pod
	// number of running pods for each deployment
	// (deployments id) -> (number of running pods of that deployment)
	NumberOfRunningPods map[int]int
}

func newEdgeConfig() *EdgeConfig {
	return &EdgeConfig{
		DeploymentIdToDeployment: make(map[int]*Deployment),

		Resources: mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil),
	}
}

func newEdgeState() *EdgeState {
	return &EdgeState{
		Config:        newEdgeConfig(),
		UsedResources: mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil),
	}
}

func NewClusterState() *ClusterState {
	return &ClusterState{
		Edge:                newEdgeState(),
		Cloud:               &CloudState{},
		PodsMap:             make(map[int]*Pod),
		NodeResourcesUsed:   make(map[int]*mat.VecDense),
		NumberOfRunningPods: make(map[int]int),
	}
}

// Following methods are for building scheduler assumption of
// cluster's static properties, if this methods are called
// in the middle of scheduler's execution it may cause unexpected
// behavior.

func (ec *EdgeConfig) AddDeployment(deployment *Deployment) bool {

	if _, ok := ec.DeploymentIdToDeployment[deployment.Id]; ok {
		return false
	}

	ec.DeploymentIdToDeployment[deployment.Id] = deployment
	ec.Deployments = append(ec.Deployments, deployment)

	return true
}

func (c *ClusterState) AddNode(n *Node, where string) {

	if where == "cloud" {
		c.Cloud.Nodes = append(c.Cloud.Nodes, n)

		log.Info().Msg("added to cloud")
		return
	}
	c.Edge.Config.Nodes = append(c.Edge.Config.Nodes, n)
	utils.SAddVec(c.Edge.Config.Resources, n.Resources)

	c.NodeResourcesUsed[n.Id] = mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil)

	log.Info().Msg("added to edge")
}

// Following methods are for changing scheduler's point of view
// of cluster's dynamic properties.
func (c *ClusterState) DeployEdge(pod *Pod, node *Node) error {
	log.Info().Msgf(
		"deploying pod %d on node %d which is on edge",
		pod.Id,
		node.Id,
	)

	if utils.LThan(utils.SubVec(node.Resources, c.NodeResourcesUsed[node.Id]), pod.Deployment.ResourcesRequired) {
		return fmt.Errorf("not enough resources for pod %d to be deployed on %d", pod.Id, node.Id)
	}

	c.Edge.Pods = append(c.Edge.Pods, pod)
	pod.Node = node

	utils.SAddVec(c.NodeResourcesUsed[node.Id], pod.Deployment.ResourcesRequired)
	utils.SAddVec(c.Edge.UsedResources, pod.Deployment.ResourcesRequired)

	c.PodsMap[pod.Id] = pod

	return nil
}

func (c *ClusterState) DeployCloud(pod *Pod) {
	log.Info().Msgf(
		"deploying pod %d to cloud",
		pod.Id,
	)

	if len(c.Cloud.Nodes) > 0 {
		target := c.Cloud.Nodes[rand.Intn(len(c.Cloud.Nodes))]
		pod.Node = target
	}

	c.PodsMap[pod.Id] = pod

	c.Cloud.Pods = append(c.Cloud.Pods, pod)
}

func (c *ClusterState) RemovePod(pod *Pod) bool {
	log.Info().Msgf("removing pod %d", pod.Id)

	ret := c.RemovePodEdge(pod)
	if !ret {
		ret = c.RemovePodCloud(pod)
	}

	pod.Node = nil
	delete(c.PodsMap, pod.Id)

	return ret
}

func (c *ClusterState) RemovePodCloud(pod *Pod) bool {
	cpod_ind := -1
	for ind, cpod := range c.Cloud.Pods {
		if cpod.Id == pod.Id {
			cpod_ind = ind
			break
		}
	}

	if cpod_ind == -1 {
		return false
	}

	pod.Node = nil
	c.Cloud.Pods[cpod_ind] = c.Cloud.Pods[len(c.Cloud.Pods)-1]
	c.Cloud.Pods = c.Cloud.Pods[:len(c.Cloud.Pods)-1]

	return true
}

func (c *ClusterState) RemovePodEdge(pod *Pod) bool {
	pod_ind := -1
	for ind, epod := range c.Edge.Pods {
		if epod.Id == pod.Id {
			pod_ind = ind
			break
		}
	}

	if pod_ind == -1 {
		return false
	}

	c.Edge.Pods[pod_ind] = c.Edge.Pods[len(c.Edge.Pods)-1]
	c.Edge.Pods = c.Edge.Pods[:len(c.Edge.Pods)-1]

	node := pod.Node
	utils.SSubVec(c.NodeResourcesUsed[node.Id], pod.Deployment.ResourcesRequired)
	utils.SSubVec(c.Edge.UsedResources, pod.Deployment.ResourcesRequired)

	return true
}

// Following methods are some utility methods for having
// a quick access to some data in cluster's state.
// or getting some common query form cluster and ...

// Returns [max(r) for each r in n for each n in all nodes]
// Used for normalization purposes
func (ec *EdgeConfig) GetMaximumResources() *mat.VecDense {
	ret := mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil)
	for _, node := range ec.Nodes {
		for i := 0; i < node.Resources.Len(); i++ {
			ret.SetVec(i, math.Max(ret.AtVec(i), node.Resources.AtVec(i)))
		}
	}

	return ret
}

// Returns a deep copy of the cluster's state.
// Deployment and Node objects are being shallow copied
// but the Pods are being deep copied.
func (c *ClusterState) Clone() *ClusterState {
	ret := NewClusterState()
	for _, deployment := range c.Edge.Config.Deployments {
		ret.Edge.Config.AddDeployment(deployment)
	}

	for _, node := range c.Edge.Config.Nodes {
		ret.AddNode(node, "edge")
	}
	for _, node := range c.Cloud.Nodes {
		ret.AddNode(node, "cloud")
	}

	for _, pod := range c.Edge.Pods {
		ret.DeployEdge(&Pod{
			Id:         pod.Id,
			Deployment: pod.Deployment,
			Node:       pod.Node,
			Status:     pod.Status,
		}, pod.Node)

		if pod.Status == RUNNING {
			ret.NumberOfRunningPods[pod.Deployment.Id]++
		}
	}
	for _, pod := range c.Cloud.Pods {
		ret.DeployCloud(&Pod{
			Id:         pod.Id,
			Deployment: pod.Deployment,
			Node:       pod.Node,
			Status:     pod.Status,
		})

		if pod.Status == RUNNING {
			ret.NumberOfRunningPods[pod.Deployment.Id]++
		}
	}

	return ret
}

// Returns a mapping of [(node id) -> (node object)].
func (c *ClusterState) GetNodeIdToNode() map[int]*Node {
	nodeIdToNode := make(map[int]*Node)

	allNodes := c.Edge.Config.Nodes
	allNodes = append(allNodes, c.Cloud.Nodes...)

	for _, node := range allNodes {
		nodeIdToNode[node.Id] = node
	}

	return nodeIdToNode
}

// Returns a mapping of [(node id) -> (node's used resource vector)]
func (c *ClusterState) GetNodesResourcesRemained() map[int]*mat.VecDense {
	NodesResourcesRemained := make(map[int]*mat.VecDense)
	// TODO change it to all nodes
	for _, node := range c.Edge.Config.Nodes {
		resources := mat.NewVecDense(node.Resources.Len(), nil)
		resources.SubVec(node.Resources, c.NodeResourcesUsed[node.Id])
		NodesResourcesRemained[node.Id] = resources
	}

	return NodesResourcesRemained
}

// Returns a string, a simple description of
// the cluster's state.
func (c *ClusterState) Display() string {
	repr := ""

	repr += "DEPLOYMENTS:\n"
	for _, deployment := range c.Edge.Config.Deployments {
		deploymentDesc := fmt.Sprintf(
			"{deployment %d (%f, %f)} share %f, number of running pods: %d",
			deployment.Id,
			deployment.ResourcesRequired.AtVec(0),
			deployment.ResourcesRequired.AtVec(1),
			deployment.EdgeShare,
			c.NumberOfRunningPods[deployment.Id],
		)
		repr += deploymentDesc
		repr += "\n"
	}
	repr += "\nSHOWING CLUSTER STATE:\n\n\n"
	repr += "========{\n"
	repr += "EDGE NODES:\n"
	for _, node := range c.Edge.Config.Nodes {
		nodeDesc := ""
		nodeDesc += fmt.Sprintf("{node %d (%f, %f)}: ", node.Id, node.Resources.AtVec(0), node.Resources.AtVec(1))
		for _, pod := range c.Edge.Pods {
			if pod.Node.Id == node.Id {
				nodeDesc += fmt.Sprintf(
					"{pod %d (%f, %f)} || ",
					pod.Id,
					pod.Deployment.ResourcesRequired.AtVec(0),
					pod.Deployment.ResourcesRequired.AtVec(1),
				)
			}
		}

		repr += nodeDesc
		repr += "\n"
	}
	repr += "\nCLOUD NODES:\n"

	// FIXME duplication
	for _, node := range c.Cloud.Nodes {
		nodeDesc := ""
		nodeDesc += fmt.Sprintf("{node %d (%f, %f)}: ", node.Id, node.Resources.AtVec(0), node.Resources.AtVec(1))
		for _, pod := range c.Cloud.Pods {
			if pod.Node.Id == node.Id {
				nodeDesc += fmt.Sprintf(
					"{pod %d (%f, %f)} || ",
					pod.Id,
					pod.Deployment.ResourcesRequired.AtVec(0),
					pod.Deployment.ResourcesRequired.AtVec(1),
				)
			}
		}

		repr += nodeDesc
		repr += "\n"
	}

	repr += "========}\n"

	return repr
}
