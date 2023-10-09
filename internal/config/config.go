package config

type GeneralConfig struct {
	Name                 string `yaml:"name"`
	Namespace            string `yaml:"namespace"`
	ResourceCount        int    `yaml:"resource_count"`
	ConnectorKind        string `yaml:"connector"`
	MaximumMigrations    int    `yaml:"maximum_migrations"`
	MaximumCloudOffload  int    `yaml:"maximum_cloud_offload"`
	FlushPeriodDuration  int    `yaml:"flush_period_duration"`  // ms
	CloudSuggestDuration int    `yaml:"cloud_suggest_duration"` // ms
	HealthCheckDuration  int    `yaml:"health_check_duration"`  // ms
	RecoverRetryDuration int    `yaml:"recover_retry_duration"` // ms
	BatchSize            int    `yaml:"batch_size"`
}

var SchedulerGeneralConfig GeneralConfig

// General constants:
const MB = 1e6
