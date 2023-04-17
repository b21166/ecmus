package model

import "gonum.org/v1/gonum/mat"

type Deployment struct {
	Id                int
	ImageSize         int
	ResourcesRequired *mat.VecDense

	// TODO score rule
	Weight float64
}

type Node struct {
	Id        int
	Resources *mat.VecDense
}

type Pod struct {
	Id         int
	Deployment *Deployment
	Node       *Node
}
