// Because it is a testing package, no errors are returned,
// all problems cause a panic.

package testing_tool

import (
	"fmt"
	"math"
	"sort"

	"github.com/amsen20/ecmus/internal/model"
	"gonum.org/v1/gonum/mat"
)

type NodeDesc struct {
	Cpu    float64
	Memory float64
}

type DeploymentDesc struct {
	Name      string
	Cpu       float64
	Memory    float64
	EdgeShare float64
}

type Builder struct {
	deployments    map[string]*model.Deployment
	deploymentName map[int]string
	lastPodId      int
	lastNodeId     int
}

func New() *Builder {
	return &Builder{
		deployments:    make(map[string]*model.Deployment),
		deploymentName: make(map[int]string),
	}
}

func (builder *Builder) ImportDeployments(deploymentsDesc []*DeploymentDesc) {
	for ind, deploymentDesc := range deploymentsDesc {
		deployment := &model.Deployment{
			Id:                ind,
			ResourcesRequired: mat.NewVecDense(2, []float64{deploymentDesc.Cpu, deploymentDesc.Memory}),
			EdgeShare:         deploymentDesc.EdgeShare,
		}
		builder.deployments[deploymentDesc.Name] = deployment
		builder.deploymentName[deployment.Id] = deploymentDesc.Name
	}
}

func (builder *Builder) GetPods(podsDesc []string) []*model.Pod {
	pods := make([]*model.Pod, 0)
	for _, podDesc := range podsDesc {
		deployment, ok := builder.deployments[podDesc]
		if !ok {
			panic(fmt.Sprintf("there is no deployment named %s", podDesc))
		}

		pod := &model.Pod{
			Id:         builder.lastPodId,
			Deployment: deployment,
			Status:     model.RUNNING,
		}
		builder.lastPodId += 1
		pods = append(pods, pod)
	}

	return pods
}

func (builder *Builder) GetCluster(edge map[*NodeDesc][]string, cloudPodsDesc []string) *model.ClusterState {
	clusterState := model.NewClusterState()

	for _, deployment := range builder.deployments {
		clusterState.Edge.Config.AddDeployment(deployment)
	}

	for nodeDesc, podsDesc := range edge {
		node := &model.Node{
			Id:        builder.lastNodeId,
			Resources: mat.NewVecDense(2, []float64{nodeDesc.Cpu, nodeDesc.Memory}),
		}
		builder.lastNodeId += 1
		clusterState.AddNode(node, "edge")

		for _, pod := range builder.GetPods(podsDesc) {
			err := clusterState.DeployEdge(pod, node)
			if err != nil {
				panic(err)
			}
		}
	}

	cloudNode := &model.Node{
		Id:        builder.lastNodeId,
		Resources: mat.NewVecDense(2, []float64{math.Inf(1), math.Inf(1)}),
	}
	builder.lastNodeId += 1
	clusterState.AddNode(cloudNode, "cloud")

	for _, pod := range builder.GetPods(cloudPodsDesc) {
		clusterState.DeployCloud(pod)
	}

	return clusterState
}

func (builder *Builder) Expect(got *model.ClusterState, wantEdge map[*NodeDesc][]string, wantCloudPodsDesc []string) {
	gotDeploymentOccurrences := make(map[string][]*NodeDesc)
	wantDeploymentOccurrences := make(map[string][]*NodeDesc)

	for _, pod := range got.Edge.Pods {
		key := builder.deploymentName[pod.Deployment.Id]
		gotDeploymentOccurrences[key] = append(gotDeploymentOccurrences[key], &NodeDesc{
			Cpu:    pod.Node.Resources.AtVec(0),
			Memory: pod.Node.Resources.AtVec(1),
		})
	}
	for _, pod := range got.Cloud.Pods {
		key := builder.deploymentName[pod.Deployment.Id]
		gotDeploymentOccurrences[key] = append(gotDeploymentOccurrences[key], &NodeDesc{
			Cpu:    0,
			Memory: 0,
		})
	}

	for nodeDesc, podsDesc := range wantEdge {
		for _, podDesc := range podsDesc {
			wantDeploymentOccurrences[podDesc] = append(wantDeploymentOccurrences[podDesc], nodeDesc)
		}
	}
	for _, podDesc := range wantCloudPodsDesc {
		wantDeploymentOccurrences[podDesc] = append(wantDeploymentOccurrences[podDesc], &NodeDesc{
			Cpu:    0,
			Memory: 0,
		})
	}

	for deployment, wantOccurrences := range wantDeploymentOccurrences {
		sort.Sort(&PodOnNodeOccurrencesSorter{
			objects: wantOccurrences,
		})

		gotOccurrences, ok := gotDeploymentOccurrences[deployment]
		if !ok {
			panic(fmt.Errorf("expected %s in got, but it wasn't", deployment))
		}
		delete(gotDeploymentOccurrences, deployment)

		sort.Sort(&PodOnNodeOccurrencesSorter{
			objects: gotOccurrences,
		})

		if len(gotOccurrences) != len(wantOccurrences) {
			panic(fmt.Errorf("got and want lengths are not equal"))
		}
		for i := range wantOccurrences {
			if wantOccurrences[i].Cpu != gotOccurrences[i].Cpu ||
				wantOccurrences[i].Memory != gotOccurrences[i].Memory {
				panic(fmt.Errorf("got %v, wanted %v", *wantOccurrences[i], *gotOccurrences[i]))
			}
		}
	}

	if len(gotDeploymentOccurrences) != 0 {
		panic(fmt.Errorf("got has more kinds of deployments than want"))
	}
}
