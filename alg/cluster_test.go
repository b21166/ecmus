package alg

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/model/testing_tool"
	"gopkg.in/yaml.v2"
)

func setUp() {
	yamlFile, err := ioutil.ReadFile("../config.yaml")
	if err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	if err := yaml.UnmarshalStrict(yamlFile, &config.SchedulerGeneralConfig); err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

}

func TestMakeDecisionForNewPods(t *testing.T) {
	setUp()
	builder := testing_tool.New()
	builder.ImportDeployments([]*testing_tool.DeploymentDesc{
		{Name: "A", Cpu: 1, Memory: 1.5, EdgeShare: 1},
		{Name: "B", Cpu: 1, Memory: 2, EdgeShare: 1},
	})

	t.Run("EmptyEdge", func(t *testing.T) {
		clusterState := builder.GetCluster(
			map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {},
			},
			[]string{},
		)

		decision := MakeDecisionForNewPods(clusterState, builder.GetPods([]string{"A", "B"}), true)
		TestingApplyDecision(clusterState, decision)

		builder.Expect(
			clusterState, map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {"A", "B"},
			},
			[]string{},
		)
	})

}

func TestComprehensiveScenario(t *testing.T) {
	setUp()
	builder := testing_tool.New()
	builder.ImportDeployments([]*testing_tool.DeploymentDesc{
		{Name: "A", Cpu: 1, Memory: 2, EdgeShare: 0.5},
		{Name: "B", Cpu: 1, Memory: 1, EdgeShare: 0.5},
		{Name: "C", Cpu: 0.5, Memory: 1, EdgeShare: 1},
		{Name: "D", Cpu: 2, Memory: 4, EdgeShare: 1},
	})

	clusterState := builder.GetCluster(
		map[*testing_tool.NodeDesc][]string{
			{Cpu: 2, Memory: 4}: {},
			{Cpu: 2, Memory: 2}: {},
			{Cpu: 2, Memory: 3}: {},
		},
		[]string{},
	)

	decision := MakeDecisionForNewPods(clusterState, builder.GetPods([]string{"A", "A", "B", "B"}), true)
	TestingApplyDecision(clusterState, decision)

	t.Run("Init stage", func(t *testing.T) {
		builder.Expect(
			clusterState, map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {"A", "A"},
				{Cpu: 2, Memory: 2}: {"B", "B"},
				{Cpu: 2, Memory: 3}: {},
			},
			[]string{},
		)
	})

	decision = MakeDecisionForNewPods(clusterState, builder.GetPods([]string{"C", "C", "B"}), true)
	TestingApplyDecision(clusterState, decision)

	t.Run("New pods", func(t *testing.T) {
		builder.Expect(
			clusterState, map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {"A", "A"},
				{Cpu: 2, Memory: 2}: {"B", "B"},
				{Cpu: 2, Memory: 3}: {"C", "C", "B"},
			},
			[]string{},
		)
	})

	deletePod := func(deploymentName string) {
		var toDeletePod *model.Pod
		for _, pod := range clusterState.Edge.Pods {
			if pod.Deployment.Id == builder.Deployments[deploymentName].Id {
				toDeletePod = pod
				break
			}
		}

		if !clusterState.RemovePod(toDeletePod) {
			t.Fatalf("pod didn't remove successfully")
		}

	}

	deletePod("B")
	decision = MakeDecisionForNewPods(clusterState, builder.GetPods([]string{"D"}), true)
	TestingApplyDecision(clusterState, decision)

	t.Run("Pod deletion", func(t *testing.T) {
		builder.Expect(
			clusterState, map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {"D"},
				{Cpu: 2, Memory: 2}: {"A"},
				{Cpu: 2, Memory: 3}: {"C", "C", "B"},
			},
			[]string{"A", "B"},
		)
	})

	deletePod("D")
	TestingApplySuggestion(clusterState, SuggestCloudToEdge(clusterState))
	TestingApplySuggestion(clusterState, SuggestCloudToEdge(clusterState))

	t.Run("Cloud to edge", func(t *testing.T) {
		builder.Expect(
			clusterState, map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {"A", "B"},
				{Cpu: 2, Memory: 2}: {"A"},
				{Cpu: 2, Memory: 3}: {"C", "C", "B"},
			},
			[]string{},
		)
	})
}
