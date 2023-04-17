package alg

import "github.com/amsen20/ecmus/internal/model"

type podSorter struct {
	pods []*model.Pod
	by   func(*model.Pod) float64
}

func (ps *podSorter) Len() int {
	return len(ps.pods)
}

func (ps *podSorter) Swap(i, j int) {
	ps.pods[i], ps.pods[j] = ps.pods[j], ps.pods[i]
}

func (ps *podSorter) Less(i, j int) bool {
	return ps.by(ps.pods[i]) < ps.by(ps.pods[j])
}
