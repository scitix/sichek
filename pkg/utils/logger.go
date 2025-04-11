package utils

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func InitLogger(level logrus.Level, isJSON bool) {
	// set log level
	logrus.SetLevel(level)

	// set log output: stdout by default
	logrus.SetOutput(os.Stdout)

	// set formatter: support JSON format or custom text format
	if isJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			ForceColors:     true,
		})
	}
}
