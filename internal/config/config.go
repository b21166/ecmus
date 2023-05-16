package config

type GeneralConfig struct {
	Name              string `yaml:"name"`
	Namespace         string `yaml:"namespace"`
	ResourceCount     int    `yaml:"resource_count"`
	MaximumMigrations int    `yaml:"maximum_migrations"`
	ConnectorKind     string `yaml:"connector"`
}

var SchedulerGeneralConfig GeneralConfig
