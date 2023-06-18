package alg

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model/testing_tool"
	"gopkg.in/yaml.v2"
)

func setUp() *testing_tool.Builder {
	builder := testing_tool.New()
	builder.ImportDeployments([]*testing_tool.DeploymentDesc{
		{Name: "A", Cpu: 1, Memory: 1.5, EdgeShare: 1},
		{Name: "B", Cpu: 1, Memory: 2, EdgeShare: 1},
	})

	yamlFile, err := ioutil.ReadFile("../config.yaml")
	if err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	if err := yaml.UnmarshalStrict(yamlFile, &config.SchedulerGeneralConfig); err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	return builder
}

func TestMakeDecisionForNewPods(t *testing.T) {
	builder := setUp()

	t.Run("EmptyEdge", func(t *testing.T) {
		clusterState := builder.GetCluster(
			map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {},
			},
			[]string{},
		)

		decision := MakeDecisionForNewPods(clusterState, builder.GetPods([]string{"A", "B"}))
		applyDecision(clusterState, decision)

		builder.Expect(
			clusterState, map[*testing_tool.NodeDesc][]string{
				{Cpu: 2, Memory: 4}: {"A", "B"},
			},
			[]string{},
		)
	})

}
