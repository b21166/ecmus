// This package is the core package of the scheduler,
// the package duty is to "model" the universe in a way
// that scheduler can understand and operate with.
//
// All concepts that the scheduler is defined and all
// the scheduler's assumptions of the cluster is defined
// in this package.
//
// Each of the following structs and their fields
// are required and sufficient information that
// the scheduler needs to operate with the cluster.
//
// If you can define the cluster in the same basic blocks as below
// and the definition does not loss some data which may affect
// the data you provided for the scheduler, the scheduler
// can operate in the cluster.
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

// For now, the scheduler can imagine pods
// only in following three states:
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

// Followings are some methods for representing
// the basic blocks.

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
