package model

import (
	"gonum.org/v1/gonum/mat"
)

type Migration struct {
	Pod  *Pod
	Node *Node
}

type FreeEdgeSolution struct {
	FreedPods  []*Pod
	Migrations []*Migration
}

type DecisionForNewPods struct {
	Score                     float64
	EdgeToCloudOffloadingPods []*Pod
	ToEdgePods                []*Pod
	ToCloudPods               []*Pod
	Migrations                []*Migration
}

type Candidate struct {
	NewResourceNeeded mat.Vector
	Solution          FreeEdgeSolution
}

type EdgePodMapping struct {
	Mapping         map[int]*Node
	DeFragmentation float64
}
