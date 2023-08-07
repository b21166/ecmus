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

type EdgeConfig struct {
	Nodes                    []*Node
	Deployments              []*Deployment
	DeploymentIdToDeployment map[int]*Deployment

	Resources *mat.VecDense
}

type EdgeState struct {
	Config *EdgeConfig

	Pods          []*Pod
	UsedResources *mat.VecDense
}

type CloudState struct {
	Nodes []*Node
	Pods  []*Pod
}

type ClusterState struct {
	Edge  *EdgeState
	Cloud *CloudState

	CandidatesList      []*Candidate
	CloudToEdgeDecision DecisionForNewPods

	NodeResourcesUsed map[int]*mat.VecDense
	PodsMap           map[int]*Pod
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
		Edge:              newEdgeState(),
		Cloud:             &CloudState{},
		PodsMap:           make(map[int]*Pod),
		NodeResourcesUsed: make(map[int]*mat.VecDense),
	}
}

func (ec *EdgeConfig) AddDeployment(deployment *Deployment) bool {
	// log.Info().Msgf("adding deployment %v", deployment)

	if _, ok := ec.DeploymentIdToDeployment[deployment.Id]; ok {
		return false
	}

	ec.DeploymentIdToDeployment[deployment.Id] = deployment
	ec.Deployments = append(ec.Deployments, deployment)

	return true
}

func (ec *EdgeConfig) GetMaximumResources() *mat.VecDense {
	ret := mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil)
	for _, node := range ec.Nodes {
		for i := 0; i < node.Resources.Len(); i++ {
			ret.SetVec(i, math.Max(ret.AtVec(i), node.Resources.AtVec(i)))
		}
	}

	return ret
}

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
	}
	for _, pod := range c.Cloud.Pods {
		ret.DeployCloud(&Pod{
			Id:         pod.Id,
			Deployment: pod.Deployment,
			Node:       pod.Node,
			Status:     pod.Status,
		})
	}

	return ret
}

func (c *ClusterState) AddNode(n *Node, where string) {
	// log.Info().Msgf("adding node %v", n)

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

func (c *ClusterState) GetNodeIdToNode() map[int]*Node {
	nodeIdToNode := make(map[int]*Node)

	allNodes := c.Edge.Config.Nodes
	allNodes = append(allNodes, c.Cloud.Nodes...)

	for _, node := range allNodes {
		nodeIdToNode[node.Id] = node
	}

	return nodeIdToNode
}

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

func (c *ClusterState) Display() string {
	repr := ""

	repr += "DEPLOYMENTS:\n"
	for _, deployment := range c.Edge.Config.Deployments {
		deploymentDesc := fmt.Sprintf(
			"{deployment %d (%f, %f)} share %f",
			deployment.Id,
			deployment.ResourcesRequired.AtVec(0),
			deployment.ResourcesRequired.AtVec(1),
			deployment.EdgeShare,
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
