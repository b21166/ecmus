package connector

import (
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/logging"
	"gopkg.in/yaml.v3"
)

type Connector interface {
	FindNodes() error
	FindDeployments() error
	GetClusterState() *model.ClusterState

	Deploy(pod *model.Pod, node *model.Node) error
	DeletePod(pod *model.Pod) (bool, error)
	WatchSchedulingEvents() (<-chan *Event, error)
}

type EventType int64

const (
	POD_CREATED EventType = iota
	POD_CHANGED
	POD_DELETED
)

type Event struct {
	EventType EventType       `yaml:"event_type"`
	Pod       *model.Pod      `yaml:"pod"`
	Node      *model.Node     `yaml:"node"`
	Status    model.PodStatus `yaml:"status"`
}

func (event *Event) String() string {
	bytes, _ := yaml.Marshal(event)
	return string(bytes[:])
}

var log = logging.Get()
