package connector

import (
	"github.com/amsen20/ecmus/internal/model"
)

type Connector interface {
	FindNodes() error
	FindDeployments() error
	GetClusterState() *model.ClusterState

	Deploy(pod *model.Pod) error
	DeletePod(pod *model.Pod) (bool, error)
	WatchSchedulingEvents() <-chan *Event
}

type EventType int64

const (
	POD_CREATED EventType = iota
	POD_CHANGED
	POD_DELETED
)

type Event struct {
	EventType EventType
	Pod       *model.Pod
	Node      *model.Node
}
