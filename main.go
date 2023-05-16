package main

import (
	"flag"
	"io/ioutil"
	"os"

	"github.com/amsen20/ecmus/internal/config"
	"github.com/amsen20/ecmus/internal/connector"
	"github.com/amsen20/ecmus/internal/scheduler"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

func main() {
	config_file_path := flag.String("config_file", "", "Path to config file")
	flag.Parse()

	yamlFile, err := ioutil.ReadFile(*config_file_path)
	if err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	if err := yaml.UnmarshalStrict(yamlFile, config.SchedulerGeneralConfig); err != nil {
		log.Err(err).Msgf("could not load config")
		os.Exit(1)
	}

	var c connector.Connector
	switch config.SchedulerGeneralConfig.ConnectorKind {
	case "fake":
		// TODO
	case "kubernetes":
		c, err = connector.NewKubeConnector("/path/to/kuberconfig")
		if err != nil {
			log.Err(err).Msg("could not init the connector")
			os.Exit(1)
		}
	default:
		log.Error().Msg("connector kind is not recognized")
	}

	scheduler.Run(c)
}
