// TODO change cluster states to edge state

package alg

import (
	"fmt"
	"math"
	"sort"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"gonum.org/v1/gonum/mat"
)

// TODO fix candidate list
// func filterCandidates(neededResources *mat.VecDense, candidates []*model.Candidate) ([]*model.Candidate, error) {
// 	return nil, nil
// }

func CalcState(c *model.ClusterState, neededResources *mat.VecDense) (model.FreeEdgeSolution, error) {
	if utils.LThan(c.Edge.Config.Resources, neededResources) {
		return model.FreeEdgeSolution{}, fmt.Errorf("resource request limit exceeded for %s", utils.ToString(neededResources))
	}

	freedPods := EvalFreePods(c, neededResources)
	migrations := CalcMigrations(c, freedPods)

	return model.FreeEdgeSolution{
		FreedPods:  freedPods,
		Migrations: migrations,
	}, nil
}

func GetMaximumScore(c *model.ClusterState, neededResources *mat.VecDense) (model.FreeEdgeSolution, error) {
	if utils.LThan(c.Edge.Config.Resources, neededResources) {
		return model.FreeEdgeSolution{}, fmt.Errorf("resource request limit exceeded for %s", utils.ToString(neededResources))
	}

	// TODO fix candidate list
	// candidates, err := filterCandidates(neededResources, c.CandidatesList)
	// if err != nil {
	// 	return model.FreeEdgeSolution{}, err
	// }

	// if len(candidates) == 0 {
	feSol, err := CalcState(c, neededResources)
	if err != nil {
		return model.FreeEdgeSolution{}, err
	}

	return feSol, nil

	// TODO fix candidate list
	// }

	// var chosenCandidate *model.Candidate
	// chosenScore := math.Inf(-1)

	// for _, candidate := range c.CandidatesList {
	// 	if candidate.Solution.Score > chosenScore {
	// 		chosenCandidate = candidate
	// 		chosenScore = candidate.Solution.Score
	// 	}
	// }

	// return chosenCandidate.Solution, nil
}

func ChooseFromPods(pods []*model.Pod, cnt int, start int, cur []*model.Pod, choices *[][]*model.Pod) {
	if cnt == 0 {
		var newChoice []*model.Pod
		copy(newChoice, cur)
		*choices = append(*choices, newChoice)

		return
	}

	for it := start; it < len(pods)-cnt+1; it++ {
		cur = append(cur, pods[it])
		ChooseFromPods(pods, cnt-1, it+1, cur, choices)
	}
}

// brute force
func GetPossiblePodChoices(c *model.ClusterState, freedPods []*model.Pod) (podChoices [][]*model.Pod) {
	freedPodIds := utils.SliceToMap(freedPods, func(pod *model.Pod) int { return pod.Id })

	remainingPods := make([]*model.Pod, 0)
	for _, pod := range c.Edge.Pods {
		if ok, isIn := freedPodIds[pod.Id]; ok && isIn {
			continue
		}
		remainingPods = append(remainingPods, pod)
	}

	for migrationCount := 1; migrationCount <= config.SchedulerGeneralConfig.MaximumMigrations; migrationCount++ {
		ChooseFromPods(remainingPods, migrationCount, 0, make([]*model.Pod, 0), &podChoices)
	}

	return
}

func FitInEdge(
	pods []*model.Pod,
	edgeConfig *model.EdgeConfig,
	nodeResourcesRemained map[int]*mat.VecDense,
	maxResources *mat.VecDense,
) (float64, map[int]*model.Node) {
	n := len(edgeConfig.Nodes)
	m := len(pods)
	dp := make([][]float64, n+1)
	par := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]float64, m+1)
		for j := range dp[i] {
			dp[i][j] = math.Inf(-1)
		}

		par[i] = make([]int, m+1)
	}
	dp[0][0] = 0
	par[0][0] = -1

	for i := 1; i < n+1; i++ {
		node := edgeConfig.Nodes[i-1]
		for j := 0; j < m+1; j++ {
			resources := mat.NewVecDense(node.Resources.Len(), nil)
			for k := j; k >= 0; k-- {
				if utils.LEThan(resources, nodeResourcesRemained[node.Id]) {
					currentDeFragmentation := utils.CalcDeFragmentation(
						utils.SubVec(nodeResourcesRemained[node.Id], resources),
						maxResources,
					)

					current := dp[i-1][k] + currentDeFragmentation
					if dp[i][j] < current {
						dp[i][j] = current
						par[i][j] = k
					}
				}

				if k > 0 {
					utils.SAddVec(resources, pods[k-1].Deployment.ResourcesRequired)
				}
			}
		}
	}

	i := n
	j := m

	for j >= 0 && dp[i][j] < 0 {
		j--
	}

	if j < 0 {
		return math.Inf(-1), make(map[int]*model.Node)
	}

	ret := make(map[int]*model.Node)

	for i > 0 {
		nextJ := par[i][j]
		if nextJ < j {
			for k := nextJ; k < j; k++ {
				ret[pods[k].Id] = edgeConfig.Nodes[i-1]
			}
		}

		j = nextJ
		i--
	}

	return dp[n][m], ret
}

func CalcMigrations(c *model.ClusterState, freedPods []*model.Pod) []*model.Migration {
	type migrations struct {
		deFragmentation float64
		migrations      []*model.Migration
	}

	maxResources := c.Edge.Config.GetMaximumResources()
	nodeResourcesRemained := c.GetNodesResourcesRemained()

	for _, pod := range freedPods {
		if pod.Node == nil {
			continue
		}

		utils.SAddVec(nodeResourcesRemained[pod.Node.Id], pod.Deployment.ResourcesRequired)
	}

	var currentDeFragmentation float64

	for _, node := range c.Edge.Config.Nodes {
		currentDeFragmentation += utils.CalcDeFragmentation(nodeResourcesRemained[node.Id], maxResources)
	}

	bestMigrations := migrations{
		deFragmentation: currentDeFragmentation,
		migrations:      nil,
	}

	calcMigrations := func(migratedPods []*model.Pod) migrations {
		for _, pod := range migratedPods {
			utils.SAddVec(nodeResourcesRemained[pod.Node.Id], pod.Deployment.ResourcesRequired)
		}

		deFragmentation, mapping := FitInEdge(migratedPods, c.Edge.Config, nodeResourcesRemained, maxResources)

		for _, pod := range migratedPods {
			utils.SSubVec(nodeResourcesRemained[pod.Node.Id], pod.Deployment.ResourcesRequired)
		}

		ret := migrations{
			deFragmentation: deFragmentation,
			migrations:      nil,
		}

		if deFragmentation < 0 || len(mapping) != len(migratedPods) {
			return ret
		}
		for _, pod := range migratedPods {
			node, ok := mapping[pod.Id]
			if !ok {
				log.Warn().Msgf("this should not happen! pod %d should find a node after migration", pod.Id)
				continue
			}
			ret.migrations = append(ret.migrations, &model.Migration{
				Pod:  pod,
				Node: node,
			})
		}

		return ret
	}

	possiblePodChoices := GetPossiblePodChoices(c, freedPods)
	for _, possiblePodChoice := range possiblePodChoices {
		for migratedPods := range utils.Permutations(possiblePodChoice) {
			currentMigrations := calcMigrations(migratedPods)
			if bestMigrations.deFragmentation < currentMigrations.deFragmentation {
				bestMigrations = currentMigrations
			}
		}
	}

	return bestMigrations.migrations
}

func EvalFreePods(c *model.ClusterState, leastResource *mat.VecDense) []*model.Pod {
	PodsOfNode := make(map[int][]*model.Pod)
	for _, node := range c.Edge.Config.Nodes {
		PodsOfNode[node.Id] = make([]*model.Pod, 0)
	}

	for _, pod := range c.Edge.Pods {
		PodsOfNode[pod.Node.Id] = append(PodsOfNode[pod.Node.Id], pod)
	}

	qosResult, err := CalcNumberOfQosSatisfactions(c.Edge.Config, c.Cloud.Pods, c.Edge.Pods, nil, nil)
	if err != nil {
		log.Err(err).Send()

		return nil
	}

	maximumResources := c.Edge.Config.GetMaximumResources()

	scoreOfFreeingPod := func(pod *model.Pod) float64 {
		var score float64
		fragmentation := utils.CalcDeFragmentation(pod.Deployment.ResourcesRequired, maximumResources)
		info := qosResult.DeploymentsQoS[pod.Deployment.Id]
		score = QoS(
			float64(info.NumberOfPodOnEdge-1)/float64(info.NumberOfPods), pod.Deployment.EdgeShare,
		) - QoS(
			float64(info.NumberOfPodOnEdge)/float64(info.NumberOfPods), pod.Deployment.EdgeShare,
		)
		score /= fragmentation

		return score
	}

	needToFreeResources := utils.SubVec(leastResource, utils.SubVec(c.Edge.Config.Resources, c.Edge.UsedResources))

	edgePods := make([]*model.Pod, len(c.Edge.Pods))
	copy(edgePods, c.Edge.Pods)

	currentFreedResources := mat.NewVecDense(needToFreeResources.Len(), nil)
	freedPods := make([]*model.Pod, 0)

	for i := 0; i < len(edgePods) && utils.LThan(currentFreedResources, needToFreeResources); i++ {
		sort.Sort(&ReverseSorter[model.Pod]{
			objects: edgePods[i:],
			by:      scoreOfFreeingPod,
		})

		utils.SAddVec(currentFreedResources, edgePods[i].Deployment.ResourcesRequired)
		qosResult.DeploymentsQoS[edgePods[i].Deployment.Id].NumberOfPodOnEdge -= 1
		freedPods = append(freedPods, edgePods[i])
	}

	return freedPods
}
