package model

import (
	"gonum.org/v1/gonum/mat"
)

type Migration struct {
	Pod  *Pod
	Node *Node
}

type FreeEdgeSolution struct {
	Score      float64
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
