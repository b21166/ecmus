package logging

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	logger zerolog.Logger
	once   sync.Once
)

func Get() zerolog.Logger {
	once.Do(func() {
		logLevel := zerolog.DebugLevel
		if os.Getenv("NO_DEBUG") != "" {
			logLevel = zerolog.InfoLevel
		}

		console := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}

		logger = zerolog.New(console).Level(logLevel).With().Timestamp().Caller().Logger()
	})

	return logger
}
