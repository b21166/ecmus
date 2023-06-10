// TODO change cluster states to edge state

package alg

import (
	"fmt"
	"math"
	"sort"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/utils"
	"github.com/emirpasic/gods/trees/binaryheap"
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
			for j := range dp[i] {
				dp[i][j] = math.Inf(-1)
			}

			par[i] = make([]int, m+1)
		}
		dp[0][0] = deFragmentation
		par[0][0] = -1

		for i := 1; i < n+1; i++ {
			node := c.Edge.Config.Nodes[i]
			for j := 0; j < m+1; j++ {
				resources := mat.NewVecDense(node.Resources.Len(), nil)
				for k := j; k >= 0; k-- {
					if utils.LEThan(resources, nodeResourcesRemained[node.Id]) {
						currentDeFragmentation := utils.CalcDeFragmentation(
							utils.SubVec(nodeResourcesRemained[node.Id], resources),
							maxResources,
						) - utils.CalcDeFragmentation(
							nodeResourcesRemained[node.Id],
							maxResources,
						)

						current := dp[i-1][k] + currentDeFragmentation
						if dp[i][k] < current {
							dp[i][j] = current
							par[i][j] = k
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
	FRAGMENTATION_FREEING_COEFFICIENT float64 = 1
	QOS_FREEING_COEFFICIENT           float64 = 2
)

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
		score += fragmentation * FRAGMENTATION_FREEING_COEFFICIENT
		info := qosResult.DeploymentsQoS[pod.Deployment.Id]
		if float64(info.NumberOfPodOnEdge)/float64(info.NumberOfPods) >= pod.Deployment.EdgeShare &&
			float64(info.NumberOfPodOnEdge-1)/float64(info.NumberOfPods) < pod.Deployment.EdgeShare {
			score -= 1 / float64(len(c.Edge.Config.Deployments)) * QOS_FREEING_COEFFICIENT
		}

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

func MapPodsToEdge(c *model.ClusterState, pods []*model.Pod) map[int]*model.Node {
	maximumResources := c.Edge.Config.GetMaximumResources()
	nodeToResRem := c.GetNodesResourcesRemained()

	podComparator := func(a, b interface{}) int {
		podA := a.(*model.Pod)
		podB := b.(*model.Pod)

		normA := utils.CalcDeFragmentation(podA.Deployment.ResourcesRequired, maximumResources)
		normB := utils.CalcDeFragmentation(podB.Deployment.ResourcesRequired, maximumResources)

		if normA < normB {
			// move A further
			return 1
		}
		if normA == normB {
			return 0
		}
		// move B further
		return -1
	}

	nodeComparator := func(a, b interface{}) int {
		nodeA := a.(*model.Node)
		nodeB := b.(*model.Node)

		normA := utils.CalcDeFragmentation(nodeToResRem[nodeA.Id], maximumResources)
		normB := utils.CalcDeFragmentation(nodeToResRem[nodeB.Id], maximumResources)

		if normA < normB {
			// move B further
			return -1
		}
		if normA == normB {
			return 0
		}
		// move A further
		return 1
	}

	ordererPods := binaryheap.NewWith(podComparator)
	ordererNodes := binaryheap.NewWith(nodeComparator)

	for _, pod := range pods {
		ordererPods.Push(pod)
	}

	nodes := make([]*model.Node, len(c.Edge.Config.Nodes))
	copy(nodes, c.Edge.Config.Nodes)

	podsMapping := make(map[int]*model.Node)
	for !ordererPods.Empty() {
		firstPod, _ := ordererPods.Pop()
		pod := firstPod.(*model.Pod)

		nodes := make([]*model.Node, 0)
		for !ordererNodes.Empty() {
			firstNode, _ := ordererNodes.Pop()
			node := firstNode.(*model.Node)
			nodes = append(nodes, node)

			if utils.LEThan(pod.Deployment.ResourcesRequired, nodeToResRem[node.Id]) {
				podsMapping[pod.Id] = node
			}
		}

		for _, node := range nodes {
			ordererNodes.Push(node)
		}
	}

	return podsMapping
}
