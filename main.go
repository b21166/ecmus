package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/gui"
	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/scheduler"
	"github.com/amsen20/ecmus/logging"
	"gopkg.in/yaml.v2"
)

var log = logging.Get()

func main() {
	config_file_path := flag.String("config_file", "", "Path to config file")
	flag.Parse()

	fmt.Println(*config_file_path)
	yamlFile, err := ioutil.ReadFile(*config_file_path)
	if err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	if err := yaml.UnmarshalStrict(yamlFile, &config.SchedulerGeneralConfig); err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	var c connector.Connector
	clusterState := model.NewClusterState()

	switch config.SchedulerGeneralConfig.ConnectorKind {
	case "const":
		c = connector.NewConstantConnector(clusterState)
	case "kubernetes":
		c, err = connector.NewKubeConnector("/home/amirhossein/.kube/config", clusterState)
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

	schedulerBridge, err := sched.Run(schedulerContext)
	if err != nil {
		log.Err(err).Msg("could not run scheduler")
		os.Exit(1)
	}

	gui.SetUp(schedulerBridge)
	gui.Run()
}
