package config

// Scheduler's general config, which is a
// singleton object read at the first of execution
// from a yaml file and used all across the code.
type generalConfig struct {
	// Scheduler's name, important for the connector
	Name string `yaml:"name"`
	// Scheduler's namespace of events, important for the connector
	Namespace string `yaml:"namespace"`
	// Number of resources of each node provides and
	// the scheduler should care, for tests and current evaluation
	// it is 2 (CPU and memory).
	ResourceCount int `yaml:"resource_count"`
	// The connector's name that the scheduler should connect to
	// For now it is either kubernetes or const
	ConnectorKind string `yaml:"connector"`
	// The connector config path.
	ConnectorConfigPath string `yaml:"connector_config"`
	// Maximum number of migrations in a single decision of the scheduler,
	// It is important to keep this number low.
	MaximumMigrations int `yaml:"maximum_migrations"`
	// Maximum number of pods that can be chosen from cloud to
	// be moved to edge in a single cloud suggestion of the scheduler.
	MaximumCloudOffload int `yaml:"maximum_cloud_offload"`
	// The duration between every scheduler decision to flush all
	// buffered pending pods and make decision about them.
	FlushPeriodDuration int `yaml:"flush_period_duration"` // ms
	// The duration between every scheduler suggestion to choose
	// some of the pods in cloud and move them to edge.
	CloudSuggestDuration int `yaml:"cloud_suggest_duration"` // ms
	// The duration between every health check of the scheduler,
	// if the health check fails scheduler perception of cluster
	// status will be refreshed.
	HealthCheckDuration int `yaml:"health_check_duration"` // ms
	// The duration between every attempt of the scheduler to
	// recover itself AFTER the health check's result became
	// negative.
	RecoverRetryDuration int `yaml:"recover_retry_duration"` // ms
	// Each decision of the scheduler will be about a batch
	// of the pods in buffer with a fixed maximum size.
	BatchSize int `yaml:"batch_size"`
}

// Shared scheduler's general config object.
var SchedulerGeneralConfig generalConfig

// General constants:
const MB = 1e6
