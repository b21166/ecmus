package logging

import (
	"fmt"
	"os"
	"strings"
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
		logLevel := zerolog.InfoLevel
		if os.Getenv("DEBUG") != "" {
			logLevel = zerolog.DebugLevel
		}

		console := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			FormatLevel: func(i interface{}) string {
				return strings.ToUpper(fmt.Sprintf("|%-5s|", i))
			},
			FormatMessage: func(i interface{}) string {
				return fmt.Sprintf(" %s ", i)
			},
			FormatFieldName: func(i interface{}) string {
				return fmt.Sprintf("%s: ", i)
			},
			FormatFieldValue: func(i interface{}) string {
				return fmt.Sprintf("%s", i)
			},
		}

		logger = zerolog.New(console).Level(logLevel).With().Timestamp().Caller().Logger()
	})

	return logger
}
