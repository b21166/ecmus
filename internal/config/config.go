package config

type GeneralConfig struct {
	Name                 string `yaml:"name"`
	Namespace            string `yaml:"namespace"`
	ResourceCount        int    `yaml:"resource_count"`
	MaximumMigrations    int    `yaml:"maximum_migrations"`
	MaximumCloudOffload  int    `yaml:"maximum_cloud_offload"`
	ConnectorKind        string `yaml:"connector"`
	DaemonPeriodDuration int    `yaml:"daemon_period_duration"` // ms
}

var SchedulerGeneralConfig GeneralConfig

// General constants:
const MB = 1e6
