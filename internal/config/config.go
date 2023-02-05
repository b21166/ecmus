package config

var SchedulerGeneralConfig struct {
	Name              string
	Namespace         string
	ResourceConfig    int
	MaximumMigrations int
}
