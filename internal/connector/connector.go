package connector

import (
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/logging"
	"gopkg.in/yaml.v3"
)

// This interface should be implemented for any connector
// so that scheduler can communicate through that connector
// with the cluster.
type Connector interface {
	// Methods are for initializing scheduler's
	// view point of cluster.
	// MUST call these methods in
	// the same order is below.
	FindNodes() error
	FindDeployments() error
	GetClusterState() *model.ClusterState

	// This method is called when scheduler wants to
	// forget everything it knows about pods and know
	// current status of the cluster.
	// This method also returns a list of pending pods.
	SyncPods() ([]*model.Pod, error)

	// Main methods for scheduler to able to
	// "schedule" and "de-schedule" pods.
	Deploy(pod *model.Pod, node *model.Node) error
	DeletePod(pod *model.Pod) (bool, error)

	// Method which channel all events related
	// to the scheduler.
	WatchSchedulingEvents() (<-chan *Event, error)
}

type EventType int64

const (
	POD_CREATED EventType = iota
	POD_CHANGED
	POD_DELETED
)

// All connectors regardless of what kind of
// cluster they are connecting to should define
// the internal cluster changes through following
// event struct.
// It contains event type, the pod that event
// is related to, the node which may be related
// to the event and the status of the pod AFTER
// the event occurred.
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
