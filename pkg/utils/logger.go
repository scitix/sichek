/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig holds configuration for logger
type LogConfig struct {
	// LogFile is the path to the log file. If empty, logs will only go to stdout
	LogFile string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated
	MaxSize int // megabytes
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int // days
	// Compress determines if the rotated log files should be compressed
	Compress bool
	// AlsoOutputToStdout determines if logs should also be written to stdout in addition to file
	AlsoOutputToStdout bool
}

func InitLogger(level logrus.Level, isJSON bool) {
	InitLoggerWithConfig(level, isJSON, LogConfig{})
}

// InitLoggerWithConfig initializes logger with file rotation support
func InitLoggerWithConfig(level logrus.Level, isJSON bool, config LogConfig) {
	// set log level
	logrus.SetLevel(level)

	// set formatter: support JSON format or custom text format
	var formatter logrus.Formatter
	if isJSON {
		formatter = &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		}
	} else {
		formatter = &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			ForceColors:     true,
		}
	}

	// set log output
	var writers []io.Writer

	// If log file is specified, set up file logging with rotation
	if config.LogFile != "" {
		// Ensure the directory exists
		logDir := filepath.Dir(config.LogFile)
		if logDir != "." && logDir != "" {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				logrus.WithError(err).Errorf("failed to create log directory: %s", logDir)
				// Fallback to stdout if directory creation fails
				logrus.SetOutput(os.Stdout)
				logrus.SetFormatter(formatter)
				return
			}
		}

		// Set default values if not specified
		maxSize := config.MaxSize
		if maxSize == 0 {
			maxSize = 100 // default: 100 MB
		}
		maxBackups := config.MaxBackups
		if maxBackups == 0 {
			maxBackups = 10 // default: keep 10 backup files
		}
		maxAge := config.MaxAge
		if maxAge == 0 {
			maxAge = 30 // default: keep logs for 30 days
		}

		// Create lumberjack logger for file rotation
		fileWriter := &lumberjack.Logger{
			Filename:   config.LogFile,
			MaxSize:    maxSize,    // megabytes
			MaxBackups: maxBackups, // number of backups
			MaxAge:     maxAge,     // days
			Compress:   config.Compress,
		}
		writers = append(writers, fileWriter)

		logrus.WithFields(logrus.Fields{
			"log_file":    config.LogFile,
			"max_size":    maxSize,
			"max_backups": maxBackups,
			"max_age":     maxAge,
			"compress":    config.Compress,
		}).Info("logging to file with rotation enabled")
	}

	// If AlsoOutputToStdout is true or no log file is specified, add stdout
	if config.AlsoOutputToStdout || config.LogFile == "" {
		writers = append(writers, os.Stdout)
	}

	// Set multiple writers if we have both file and stdout
	if len(writers) > 1 {
		logrus.SetOutput(io.MultiWriter(writers...))
	} else if len(writers) == 1 {
		logrus.SetOutput(writers[0])
	} else {
		// Fallback to stdout if no writers
		logrus.SetOutput(os.Stdout)
	}

	logrus.SetFormatter(formatter)
}
