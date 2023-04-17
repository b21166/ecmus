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

func filterCandidates(neededResources *mat.VecDense, candidates []*model.Candidate) ([]*model.Candidate, error) {
	return nil, nil
}

func CalcState(c *model.ClusterState, neededResources *mat.VecDense) (*model.FreeEdgeSolution, error) {
	return nil, nil
}

func GetMaximumScore(c *model.ClusterState, oNeededResources *mat.VecDense) ([]*model.Migration, []*model.Pod, int, error) {
	if utils.LThan(c.Edge.Config.Resources, oNeededResources) {
		return nil, nil, -1, fmt.Errorf("resource request limit exceeded for %s", utils.ToString(oNeededResources))
	}

	neededResources := mat.NewVecDense(oNeededResources.Len(), nil)
	neededResources.SubVec(oNeededResources, c.ResourcesBuffer)

	candidates, err := filterCandidates(neededResources, c.CandidatesList)
	if err != nil {
		return nil, nil, -1, err
	}

	if len(candidates) == 0 {
		feSol, err := CalcState(c, neededResources)
		if err != nil {
			return nil, nil, -1, err
		}

		return feSol.Migrations, feSol.FreedPods, feSol.Score, nil
	}

	var chosenCandidate *model.Candidate
	chosenScore := int(-1e9)

	for _, candidate := range c.CandidatesList {
		if candidate.Solution.Score > chosenScore {
			chosenCandidate = candidate
			chosenScore = candidate.Solution.Score
		}
	}

	return chosenCandidate.Solution.Migrations, chosenCandidate.Solution.FreedPods, chosenCandidate.Solution.Score, nil
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

	remPods := make([]*model.Pod, 0)
	for _, pod := range c.Edge.Pods {
		if ok, isIn := freedPodIds[pod.Id]; ok && isIn {
			continue
		}
		remPods = append(remPods, pod)
	}

	for migrationCount := 1; migrationCount <= config.SchedulerGeneralConfig.MaximumMigrations; migrationCount++ {
		ChooseFromPods(remPods, migrationCount, 0, make([]*model.Pod, 0), &podChoices)
	}

	return
}

// Dynamic Programming
func CalcMigrations(c *model.ClusterState, freedPods []*model.Pod) []*model.Migration {
	possiblePodChoices := GetPossiblePodChoices(c, freedPods)

	type migrations struct {
		fragmentationInc float64
		migrations       []*model.Migration
	}

	maxResources := c.Edge.Config.GetMaximumResources()

	nodeResourcesRemained := c.GetNodesResourcesRemained()

	calcDp := func(migratedPods []*model.Pod) migrations {
		var deFragmentation float64
		deFragmentation = 0
		for _, pod := range migratedPods {
			deFragmentation -= utils.CalcDeFragmentation(nodeResourcesRemained[pod.Node.Id], maxResources)
			utils.SAddVec(nodeResourcesRemained[pod.Node.Id], pod.Deployment.ResourcesRequired)
			deFragmentation += utils.CalcDeFragmentation(nodeResourcesRemained[pod.Node.Id], maxResources)
		}

		n := len(c.Edge.Config.Nodes)
		m := len(migratedPods)
		dp := make([][]float64, n+1)
		par := make([][]int, n+1)
		for i := range dp {
			dp[i] = make([]float64, m+1)
			par[i] = make([]int, m+1)
		}
		dp[0][0] = deFragmentation
		par[0][0] = -1

		for i := 1; i < n+1; i++ {
			node := c.Edge.Config.Nodes[i]
			for j := 0; j < m+1; j++ {
				resources := mat.NewVecDense(node.Resources.Len(), nil)
				for k := i; k >= 0; k-- {
					if utils.LEThan(resources, nodeResourcesRemained[node.Id]) {
						currentDeFragmentation := utils.CalcDeFragmentation(
							utils.SubVec(nodeResourcesRemained[node.Id], resources),
							maxResources,
						) - utils.CalcDeFragmentation(
							nodeResourcesRemained[node.Id],
							maxResources,
						)

						current := dp[i-1][j] + currentDeFragmentation
						if dp[i][j] < current {
							dp[i][j] = current
							par[i][j] = j
						}
					}

					if k > 0 {
						utils.SAddVec(resources, migratedPods[k-1].Deployment.ResourcesRequired)
					}
				}
			}
		}

		for _, pod := range migratedPods {
			utils.SSubVec(nodeResourcesRemained[pod.Node.Id], pod.Deployment.ResourcesRequired)
		}

		if dp[n][m] < 0 {
			return migrations{
				fragmentationInc: 0,
				migrations:       nil,
			}
		}

		i := n
		j := m
		ret := migrations{
			fragmentationInc: dp[n][m],
			migrations:       nil,
		}
		for i > 0 {
			nextJ := par[i][j]
			if nextJ < j {
				for k := nextJ; k < j; k++ {
					ret.migrations = append(ret.migrations, &model.Migration{
						Pod:  migratedPods[k],
						Node: c.Edge.Config.Nodes[i],
					})
				}
			}

			j = nextJ
			i--
		}

		return ret
	}

	bestMigrations := migrations{
		fragmentationInc: 0,
		migrations:       nil,
	}

	for _, possiblePodChoice := range possiblePodChoices {
		for migratedPods := range utils.Permutations(possiblePodChoice) {
			currentMigrations := calcDp(migratedPods)
			if bestMigrations.fragmentationInc < currentMigrations.fragmentationInc {
				bestMigrations = currentMigrations
			}
		}
	}

	return bestMigrations.migrations
}

// TODO move it to config
const (
	FRAGMENTATION_COEFFICIENT float64 = 1
	WEIGHT_COEFFICIENT        float64 = 2
)

func EvalFreePods(c *model.ClusterState, leastResource *mat.VecDense) (float64, []*model.Pod) {
	PodsOfNode := make(map[int][]*model.Pod)
	for _, node := range c.Edge.Config.Nodes {
		PodsOfNode[node.Id] = make([]*model.Pod, 0)
	}

	var maxWeight float64
	for _, pod := range c.Edge.Pods {
		maxWeight = math.Max(maxWeight, pod.Deployment.Weight)
		PodsOfNode[pod.Node.Id] = append(PodsOfNode[pod.Node.Id], pod)
	}

	maximumResources := c.Edge.Config.GetMaximumResources()

	costOfFreeingPod := func(pod *model.Pod) float64 {
		var cost float64
		fragmentation := utils.CalcDeFragmentation(pod.Deployment.ResourcesRequired, maximumResources)
		cost += fragmentation * FRAGMENTATION_COEFFICIENT
		cost -= pod.Deployment.Weight * WEIGHT_COEFFICIENT

		return cost
	}

	needToFreeResources := utils.SubVec(c.Edge.Config.Resources, leastResource)
	for i := 0; i < needToFreeResources.Len(); i++ {
		if needToFreeResources.AtVec(i) < 0 {
			needToFreeResources.SetVec(i, 0)
		}
	}

	edgePods := make([]*model.Pod, len(c.Edge.Pods))
	copy(edgePods, c.Edge.Pods)
	sort.Sort(&podSorter{
		pods: edgePods,
		by:   costOfFreeingPod,
	})

	currentFreedResources := mat.NewVecDense(needToFreeResources.Len(), nil)
	freedPods := make([]*model.Pod, 0)
	var cost float64

	for i := 0; i < len(edgePods) && utils.LThan(currentFreedResources, needToFreeResources); i++ {
		utils.SAddVec(currentFreedResources, edgePods[i].Deployment.ResourcesRequired)
		cost += costOfFreeingPod(edgePods[i])
		freedPods = append(freedPods, edgePods[i])
	}

	return cost, freedPods
}
