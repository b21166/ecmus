package model

import (
	"github.com/amsen20/ecmus/internal/utils"
	"gonum.org/v1/gonum/mat"
	"gopkg.in/yaml.v3"
)

type Deployment struct {
	Id                int
	ResourcesRequired *mat.VecDense
	EdgeShare         float64
}

type Node struct {
	Id        int           `yaml:"id"`
	Resources *mat.VecDense `yaml:"resources"`
}

type PodStatus int

const (
	SCHEDULED PodStatus = iota
	RUNNING
	FINISHED
)

type Pod struct {
	Id         int         `yaml:"id"`
	Deployment *Deployment `yaml:"deployment"`
	Node       *Node       `yaml:"node"`
	Status     PodStatus   `yaml:"status"`
}

func (deployment *Deployment) MarshalYAML() (interface{}, error) {
	return &struct {
		Id                int     `yaml:"id"`
		ResourcesRequired string  `yaml:"resources"`
		EdgeShare         float64 `yaml:"edge_share"`
	}{
		Id:                deployment.Id,
		ResourcesRequired: utils.ToString(deployment.ResourcesRequired),
		EdgeShare:         deployment.EdgeShare,
	}, nil
}

func (node *Node) MarshalYAML() (interface{}, error) {
	return &struct {
		Id        int    `yaml:"id"`
		Resources string `yaml:"resources"`
	}{
		Id:        node.Id,
		Resources: utils.ToString(node.Resources),
	}, nil
}

func (deployment *Deployment) String() string {
	bytes, _ := yaml.Marshal(deployment)
	return string(bytes[:])
}

func (node *Node) String() string {
	bytes, _ := yaml.Marshal(node)
	return string(bytes[:])
}

func (pod *Pod) String() string {
	bytes, _ := yaml.Marshal(pod)
	return string(bytes[:])
}
