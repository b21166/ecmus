package connector

import "github.com/amsen20/ecmus/internal/model"

type Connector interface {
	FindNodes() error
	FindDeployments() error
	GetClusterState() *model.ClusterState

	WatchSchedulingEvents()
	MigratePods(pod *model.Pod, node *model.Node)
}
