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
package daemon

import (
	"context"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/scitix/sichek/cmd/command/component"
	"github.com/scitix/sichek/cmd/command/specgen"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/systemd"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/scitix/sichek/service"
)

func NewDaemonRunCmd() *cobra.Command {
	daemonRunCmd := &cobra.Command{
		Use:   "run",
		Short: "Run sichek daemon process",
		Run: func(cmd *cobra.Command, args []string) {
			_, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			// Get log file configuration
			logFile, _ := cmd.Flags().GetString("log-file")
			logMaxSize, _ := cmd.Flags().GetInt("log-max-size")
			logMaxBackups, _ := cmd.Flags().GetInt("log-max-backups")
			logMaxAge, _ := cmd.Flags().GetInt("log-max-age")
			logCompress, _ := cmd.Flags().GetBool("log-compress")
			logAlsoStdout, _ := cmd.Flags().GetBool("log-also-stdout")

			// Get log level
			logLevelStr, _ := cmd.Flags().GetString("log-level")
			logLevel := logrus.WarnLevel // default level
			if logLevelStr != "" {
				parsedLevel, err := logrus.ParseLevel(logLevelStr)
				if err != nil {
					logrus.WithField("daemon", "run").Warnf("invalid log level '%s', using default 'warn': %v", logLevelStr, err)
				} else {
					logLevel = parsedLevel
					logrus.WithField("daemon", "run").Infof("using log level: %s", logLevelStr)
				}
			}

			// Initialize logger with file rotation support
			logConfig := utils.LogConfig{
				LogFile:            logFile,
				MaxSize:            logMaxSize,
				MaxBackups:         logMaxBackups,
				MaxAge:             logMaxAge,
				Compress:           logCompress,
				AlsoOutputToStdout: logAlsoStdout,
			}
			utils.InitLoggerWithConfig(logLevel, false, logConfig)
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				cfgFile, err = specgen.EnsureSpecFile(cfgFile)
				if err != nil {
					logrus.WithField("daemon", "run").Errorf("using default cfgFile: %v", err)
				} else {
					logrus.WithField("daemon", "run").Info("using cfgFile: " + cfgFile)
				}
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				specFile, err = specgen.EnsureSpecFile(specFile)
				if err != nil {
					logrus.WithField("daemon", "run").Errorf("using default specFile: %v", err)
				} else {
					logrus.WithField("daemon", "run").Info("using specFile: " + specFile)
				}
			}

			usedComponentStr, err := cmd.Flags().GetString("enable-components")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("enable components = %v", usedComponentStr)
			}
			ignoreComponentStr, err := cmd.Flags().GetString("ignore-components")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("ignore-components = %v", ignoreComponentStr)
			}
			annoKey, err := cmd.Flags().GetString("annotation-key")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("set annotation-key %s", annoKey)
			}
			metricsPort, err := cmd.Flags().GetInt("metrics-port")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
				metricsPort = 0
			} else if metricsPort > 0 {
				logrus.WithField("daemon", "run").Infof("using metrics port from command line: %d", metricsPort)
			}

			start := time.Now()
			signals := make(chan os.Signal, 2048)
			serviceChan := make(chan service.Service, 1)

			logrus.WithField("daemon", "run").Info("starting sichek daemon service")
			done := service.HandleSignals(cancel, signals, serviceChan)
			signal.Notify(signals, service.AllowedSignals...)
			components := make(map[string]common.Component)

			componentsToCheck := component.DetermineComponentsToCheck(usedComponentStr, ignoreComponentStr, cfgFile, "daemon")
			for _, componentName := range componentsToCheck {
				if componentName == consts.ComponentNameInfiniband && !utils.IsInfinibandExist() {
					continue
				}
				if !slices.Contains(consts.DefaultComponents, componentName) {
					continue
				}
				component, err := component.NewComponent(componentName, cfgFile, specFile, nil)
				if err != nil {
					logrus.WithField("daemon", "run").Errorf("failed to create component %s: %v, skipping", componentName, err)
					continue
				}
				if component == nil {
					logrus.WithField("daemon", "run").Errorf("component %s is nil after creation, skipping", componentName)
					continue
				}
				components[componentName] = component
			}
			daemonService, err := service.NewService(components, annoKey, cfgFile, metricsPort)
			if err != nil {
				logrus.WithField("daemon", "run").Errorf("create daemon service failed: %v", err)
				return
			}
			serviceChan <- daemonService
			go daemonService.Run()

			if exist, _ := systemd.SystemctlExists(); exist {
				if err := service.NotifyReady(); err != nil {
					logrus.WithField("daemon", "run").Warn("notify is not ready")
				}
			} else {
				logrus.WithField("daemon", "run").Debug("skip sd notify as systemd not exist")
			}

			logrus.WithField("daemon", "run").Infof("sichek daemon service run succeed, take %f seconds", time.Since(start).Seconds())
			<-done
		},
	}
	daemonRunCmd.Flags().StringP("cfg", "c", "", "Path to the user config file")
	daemonRunCmd.Flags().StringP("spec", "s", "", "Path to the specification file")
	daemonRunCmd.Flags().StringP("enable-components", "E", "", "Enabled components, joined by `,`")
	daemonRunCmd.Flags().StringP("ignore-components", "I", "", "Ignored components")
	daemonRunCmd.Flags().StringP("annotation-key", "A", "", "k8s node annotation key")
	daemonRunCmd.Flags().IntP("metrics-port", "p", 0, "Prometheus metrics server port(0 means use config file)")
	daemonRunCmd.Flags().StringP("log-file", "f", "/tmp/sichek.log", "Path to log file (enables file logging with rotation)")
	daemonRunCmd.Flags().StringP("log-level", "l", "warn", "Log level (trace, debug, info, warn, error, fatal, panic)")
	daemonRunCmd.Flags().Int("log-max-size", 10, "Maximum size in megabytes of the log file before rotation")
	daemonRunCmd.Flags().Int("log-max-backups", 10, "Maximum number of old log files to retain")
	daemonRunCmd.Flags().Int("log-max-age", 10, "Maximum number of days to retain old log files")
	daemonRunCmd.Flags().Bool("log-compress", false, "Compress rotated log files")
	daemonRunCmd.Flags().Bool("log-also-stdout", false, "Also output logs to stdout in addition to file")
	return daemonRunCmd
}
