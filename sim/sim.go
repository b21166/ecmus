package sim

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/amsen20/ecmus/alg"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/model/testing_tool"
	"github.com/amsen20/ecmus/internal/utils"
	"github.com/amsen20/ecmus/logging"
)

type Frame struct {
	NewPods     []string `json:"new_pods"`
	DeletedPods []string `json:"delete_pods"`
}

var report struct {
	QoS  []float64 `json:"qos"`
	Edge []float64 `json:"edge_usage"`
}

type algorithmType int

const (
	KUBERNETES_DEFAULT algorithmType = iota
	QASRE
)

var log = logging.Get()

var (
	clusterState *model.ClusterState
	builder      *testing_tool.Builder
)

func setUpBuilder() {
	builder = testing_tool.New()
	builder.ImportDeployments([]*testing_tool.DeploymentDesc{
		{Name: "A", Cpu: 1, Memory: 2, EdgeShare: 0.5},
		{Name: "B", Cpu: 1, Memory: 1, EdgeShare: 0.5},
		{Name: "C", Cpu: 0.5, Memory: 1, EdgeShare: 1},
		{Name: "D", Cpu: 2, Memory: 4, EdgeShare: 1},
	})

	clusterState = builder.GetCluster(
		map[*testing_tool.NodeDesc][]string{
			{Cpu: 2, Memory: 4}: {},
			{Cpu: 2, Memory: 2}: {},
			{Cpu: 2, Memory: 3}: {},
		},
		[]string{},
	)
}

func deletePods(deletedPods []*model.Pod) {
pods:
	for _, pod := range deletedPods {
		for _, edgePod := range clusterState.Edge.Pods {
			if pod.Deployment.Id == edgePod.Deployment.Id {
				clusterState.RemovePod(edgePod)
				continue pods
			}
		}

		for _, cloudPod := range clusterState.Cloud.Pods {
			if pod.Deployment.Id == cloudPod.Deployment.Id {
				clusterState.RemovePod(cloudPod)
				continue pods
			}
		}

		panic("didn't find the pod")
	}
}

func kubeDefaultSchedulePods(newPods []*model.Pod) {
pods:
	for _, pod := range newPods {
		resRem := clusterState.GetNodesResourcesRemained()
		for _, node := range clusterState.Edge.Config.Nodes {
			if utils.LEThan(pod.Deployment.ResourcesRequired, resRem[node.Id]) {
				if err := clusterState.DeployEdge(pod, node); err != nil {
					panic(err)
				}

				continue pods
			}
		}

		clusterState.DeployCloud(pod)
	}
}

func Start() {
	var testingAlgorithm algorithmType

	var choice string
	fmt.Scan(&choice)
	if choice == "QASRE" {
		testingAlgorithm = QASRE
	} else {
		testingAlgorithm = KUBERNETES_DEFAULT
	}

	jsonFile, err := os.Open("./sim/scenario.json")
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()

	bytes, _ := ioutil.ReadAll(jsonFile)
	var frames []*Frame
	err = json.Unmarshal([]byte(bytes), &frames)
	if err != nil {
		fmt.Println(err.Error())
	}

	setUpBuilder()

	for ind, frame := range frames {
		log.Info().Msgf("processing frame: %d, length: %d", ind, len(frame.NewPods))
		newPods := builder.GetPods(frame.NewPods)
		deletedPods := builder.GetPods(frame.DeletedPods)

		deletePods(deletedPods)

		switch testingAlgorithm {
		case KUBERNETES_DEFAULT:
			kubeDefaultSchedulePods(newPods)
		case QASRE:
			decision := alg.MakeDecisionForNewPods(clusterState, newPods)
			alg.TestingApplyDecision(clusterState, decision)
			alg.TestingApplySuggestion(clusterState, alg.SuggestCloudToEdge(clusterState))
			alg.TestingApplySuggestion(clusterState, alg.SuggestCloudToEdge(clusterState))
		}

		qos, err := alg.CalcNumberOfQosSatisfactions(clusterState.Edge.Config, clusterState.Cloud.Pods, clusterState.Edge.Pods, nil, nil)
		if err != nil {
			panic(err)
		}

		qosSatisfied := 0
		for deploymentId, info := range qos.DeploymentsQoS {
			edgeShare := clusterState.Edge.Config.DeploymentIdToDeployment[deploymentId].EdgeShare
			if float64(info.NumberOfPods)*edgeShare <= float64(info.NumberOfPodOnEdge) {
				qosSatisfied += 1
			}
		}

		report.QoS = append(report.QoS, float64(qosSatisfied))
		report.Edge = append(report.Edge, utils.CalcDeFragmentation(clusterState.Edge.UsedResources, clusterState.Edge.Config.Resources))
	}

	for i := range report.QoS {
		fmt.Printf("%f, %f\n", report.QoS[i], report.Edge[i])
	}

	content, _ := json.MarshalIndent(report, "", " ")

	_ = ioutil.WriteFile(fmt.Sprintf("./sim/%s.json", choice), content, 0644)
}
