package model

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/utils"
	"gonum.org/v1/gonum/mat"
)

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

	ResourcesBuffer   *mat.VecDense
	NodeResourcesUsed map[int]*mat.VecDense
	PodsMap           map[int]*Pod
}

func newEdgeConfig() *EdgeConfig {
	return &EdgeConfig{
		DeploymentIdToDeployment: make(map[int]*Deployment),
	}
}

func newEdgeState() *EdgeState {
	return &EdgeState{
		Config: newEdgeConfig(),
	}
}

func NewClusterState() *ClusterState {
	return &ClusterState{
		Edge:              newEdgeState(),
		ResourcesBuffer:   mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil),
		PodsMap:           make(map[int]*Pod),
		NodeResourcesUsed: make(map[int]*mat.VecDense),
	}
}

func (ec *EdgeConfig) AddDeployment(deployment *Deployment) bool {
	if _, ok := ec.DeploymentIdToDeployment[deployment.Id]; ok {
		return false
	}

	ec.DeploymentIdToDeployment[deployment.Id] = deployment
	ec.Deployments = append(ec.Deployments, deployment)

	return true
}

func (ec *EdgeConfig) GetMaximumResources() *mat.VecDense {
	ret := mat.NewVecDense(0, nil)
	for _, node := range ec.Nodes {
		for i := 0; i < node.Resources.Len(); i++ {
			ret.SetVec(i, math.Max(ret.AtVec(i), node.Resources.AtVec(i)))
		}
	}

	return ret
}

func (c *ClusterState) Clone() *ClusterState {
	ret := NewClusterState()

	// Unsafe for using and mutating
	ret.Edge.Config = c.Edge.Config
	ret.Cloud.Nodes = c.Cloud.Nodes

	// Safe for using and mutating
	copy(ret.Edge.Pods, c.Edge.Pods)
	copy(ret.Cloud.Pods, c.Cloud.Pods)

	for nodeId, res := range c.NodeResourcesUsed {
		var newRes *mat.VecDense
		newRes.CloneFromVec(res)
		ret.NodeResourcesUsed[nodeId] = newRes
	}

	ret.ResourcesBuffer.CloneFromVec(c.ResourcesBuffer)

	return ret
}

func (c *ClusterState) AddNode(n *Node, where string) {
	if where == "cloud" {
		c.Cloud.Nodes = append(c.Cloud.Nodes, n)
		return
	}

	c.Edge.Config.Nodes = append(c.Edge.Config.Nodes, n)
	utils.SAddVec(c.Edge.Config.Resources, n.Resources)

	c.NodeResourcesUsed[n.Id] = mat.NewVecDense(config.SchedulerGeneralConfig.ResourceCount, nil)
}

func (c *ClusterState) AddToBuffer(vec *mat.VecDense) {
	utils.SAddVec(c.ResourcesBuffer, vec)
}

func (c *ClusterState) RemoveFromBuffer(vec *mat.VecDense) {
	utils.SSubVec(c.ResourcesBuffer, vec)
}

func (c *ClusterState) ResetBuffer(vec *mat.VecDense) {
	c.ResourcesBuffer.Zero()
}

func (c *ClusterState) DeployEdge(pod *Pod, node *Node) error {
	if utils.LThan(utils.SubVec(node.Resources, c.NodeResourcesUsed[node.Id]), pod.Deployment.ResourcesRequired) {
		return fmt.Errorf("not enough resources for pod %d to be deployed on %d", pod.Id, node.Id)
	}

	c.Edge.Pods = append(c.Edge.Pods, pod)
	pod.Node = node

	c.AddToBuffer(pod.Deployment.ResourcesRequired)

	utils.SAddVec(c.NodeResourcesUsed[node.Id], pod.Deployment.ResourcesRequired)
	utils.SAddVec(c.Edge.UsedResources, pod.Deployment.ResourcesRequired)

	c.PodsMap[pod.Id] = pod

	return nil
}

func (c *ClusterState) DeployCloud(pod *Pod) {
	if len(c.Cloud.Nodes) > 0 {
		target := c.Cloud.Nodes[rand.Intn(len(c.Cloud.Nodes))]
		pod.Node = target
	}

	c.PodsMap[pod.Id] = pod

	c.Cloud.Pods = append(c.Cloud.Pods, pod)
}

func (c *ClusterState) RemovePod(pod *Pod) bool {
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
	c.Cloud.Pods[cpod_ind] = c.Edge.Pods[len(c.Edge.Pods)-1]
	c.Cloud.Pods = c.Edge.Pods[:len(c.Edge.Pods)-1]

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
	c.RemoveFromBuffer(pod.Deployment.ResourcesRequired)

	node := pod.Node
	utils.SSubVec(c.NodeResourcesUsed[node.Id], pod.Deployment.ResourcesRequired)
	utils.SSubVec(c.Edge.UsedResources, pod.Deployment.ResourcesRequired)

	return true
}

func (c *ClusterState) Update(cl []*Candidate, buffer *mat.VecDense, dec DecisionForNewPods) {
	c.CandidatesList = cl
	c.CloudToEdgeDecision = dec
	c.RemoveFromBuffer(buffer)
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
