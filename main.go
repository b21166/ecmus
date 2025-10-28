package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/gui"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/scheduler"
	"github.com/amsen20/ecmus/logging"
	"github.com/amsen20/ecmus/statistics"
	"gopkg.in/yaml.v2"
)

var log = logging.Get()

func main() {
	config_file_path := flag.String("config_file", "", "Path to config file")
	flag.Parse()

	fmt.Println(*config_file_path)
	yamlFile, err := os.ReadFile(*config_file_path)
	if err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	if err := yaml.UnmarshalStrict(yamlFile, &config.SchedulerGeneralConfig); err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	// The cluster state will be shared between connector and scheduler.
	clusterState := model.NewClusterState()

	var c connector.Connector
	// Initialize the connector with the connector kind mentioned in config.
	switch config.SchedulerGeneralConfig.ConnectorKind {
	case "const":
		c = connector.NewConstantConnector(clusterState)
	case "kubernetes":
		c, err = connector.NewKubeConnector(clusterState)
		if err != nil {
			log.Err(err).Msg("could not init the connector")
			os.Exit(1)
		}
	default:
		log.Error().Msg("connector kind is not recognized")
		os.Exit(1)
	}

	sched, err := scheduler.New(clusterState, c)
	if err != nil {
		log.Err(err).Msg("could not initiate scheduler")
		os.Exit(1)
	}

	if err := sched.Start(); err != nil {
		log.Err(err).Msg("could not start scheduler")
		os.Exit(1)
	}

	schedulerContext := context.Background()

	// Scheduler's bridge is a way for other goroutines to ask
	// the scheduler for getting snapshots of the current state.
	schedulerBridge, err := sched.Run(schedulerContext)
	if err != nil {
		log.Err(err).Msg("could not run scheduler")
		os.Exit(1)
	}

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	// Setup statistics.
	statistics.Init()
	statistics.Set("restarts", 0)

	// Simple gui in web-server for checking state's status.
	gui.SetUp(schedulerBridge)
	go gui.Run()

	<-signalChannel
	log.Info().Msgf("exiting gracefully...")
	log.Info().Msgf("\n%s", statistics.Display())
}
