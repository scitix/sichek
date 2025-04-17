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
	"strings"
	"time"

	"github.com/scitix/sichek/cmd/command/component"
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

			utils.InitLogger(logrus.InfoLevel, false)
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				if cfgFile != "" {
					logrus.WithField("daemon", "run").Info("load cfgFile: " + cfgFile)
				} else {
					logrus.WithField("daemon", "run").Info("load default cfgFile...")
				}
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("daemon", "run").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("daemon", "run").Info("load default specFile...")
				}
			}

			usedComponentStr, err := cmd.Flags().GetString("enable-components")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("enable components = %v", usedComponentStr)
			}
			usedComponents := make([]string, 0)
			if len(usedComponentStr) > 0 {
				usedComponents = strings.Split(usedComponentStr, ",")
			}
			ignoreComponentStr, err := cmd.Flags().GetString("ignore-components")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("ignore-components = %v", ignoreComponentStr)
			}
			ignoredComponents := make([]string, 0)
			if len(ignoreComponentStr) > 0 {
				ignoredComponents = strings.Split(ignoreComponentStr, ",")
			}
			annoKey, err := cmd.Flags().GetString("annotation-key")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("set annotation-key %s", annoKey)
			}

			start := time.Now()
			signals := make(chan os.Signal, 2048)
			serviceChan := make(chan service.Service, 1)

			logrus.WithField("daemon", "run").Info("starting sichek daemon service")
			done := service.HandleSignals(cancel, signals, serviceChan)
			signal.Notify(signals, service.AllowedSignals...)
			components := make(map[string]common.Component)
			for _, componentName := range consts.DefaultComponents {
				if slices.Contains(ignoredComponents, componentName) {
					continue
				}
				if len(usedComponentStr) > 0 && !slices.Contains(usedComponents, componentName) {
					continue
				}
				component, err := component.NewComponent(componentName, cfgFile, specFile, nil)
				if err != nil {
					logrus.WithField("daemon", "run").Errorf("failed to create component %s: %v", componentName, err)
					continue
				}
				components[componentName] = component
			}

			daemonService, err := service.NewService(components, annoKey)
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
	daemonRunCmd.Flags().StringP("cfg", "c", "", "Path to the Infinibnad Cfg")
	daemonRunCmd.Flags().StringP("spec", "s", "", "Path to the specification file")
	daemonRunCmd.Flags().StringP("enable-components", "E", "", "Enabled components, joined by `,`")
	daemonRunCmd.Flags().StringP("ignore-components", "I", "", "Ignored components")
	daemonRunCmd.Flags().StringP("annotation-key", "A", "", "k8s node annotation key")
	return daemonRunCmd
}
