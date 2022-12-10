package alg

import (
	"fmt"

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
	// freedPodIds := utils.SliceToMap(freedPods, func(pod *model.Pod) int { return pod.Id })
	// possiblePodChoices := GetPossiblePodChoices(c, freedPods)
	// TODO
	return nil
}
