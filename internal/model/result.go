package model

import (
	"gonum.org/v1/gonum/mat"
)

type Migration struct {
	Pod  *Pod
	Node *Node
}

type FreeEdgeSolution struct {
	Score      int
	FreedPods  []*Pod
	Migrations []Migration
}

type Decision struct {
	Score          int
	OffloadingPods *Pod
	ToEdgePods     *Pod
	ToCloudPods    *Pod
	Migrations     []Migration
}

type Candidate struct {
	NewResourceNeeded mat.Vector
	Solution          FreeEdgeSolution
}
