package model

import "gonum.org/v1/gonum/mat"

type Deployment struct {
	Id                int
	ResourcesRequired *mat.VecDense

	// TODO score rule
	Weight float64
}

type Node struct {
	Id        int
	Resources *mat.VecDense
}

type PodStatus int

const (
	SCHEDULED PodStatus = iota
	RUNNING
	FINISHED
)

type Pod struct {
	Id         int
	Deployment *Deployment
	Node       *Node
	Status     PodStatus
}
