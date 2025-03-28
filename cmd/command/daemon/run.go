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
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/scitix/sichek/config"
	pkg_systemd "github.com/scitix/sichek/pkg/systemd"
	"github.com/scitix/sichek/service"
)

// NewDaemonRunCmd创建并返回用于直接运行 daemon 进程的子命令实例，配置命令的基本属性
func NewDaemonRunCmd() *cobra.Command {
	daemonRunCmd := &cobra.Command{
		Use:   "run",
		Short: "Run sichek daemon process",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

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

			used_component_str, err := cmd.Flags().GetString("enable-components")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("enable components = %v", used_component_str)
			}
			used_components := make([]string, 0)
			if len(used_component_str) > 0 {
				used_components = strings.Split(used_component_str, ",")
			}
			ignore_component_str, err := cmd.Flags().GetString("ignore-components")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Infof("ignore-components = %v", ignore_component_str)
			}
			ignored_components := make([]string, 0)
			if len(ignore_component_str) > 0 {
				ignored_components = strings.Split(ignore_component_str, ",")
			}

			var cfg *config.Config
			if cfgFile != "" {
				cfg, err = config.LoadConfigFromYaml(cfgFile)
				if err != nil {
					logrus.WithField("components", "infiniband").Error(err)
				}
			} else {
				// 默认配置
				cfg, err = config.GetDefaultConfig(used_components, ignored_components)
				if err != nil {
					logrus.WithField("daemon", "run").Error("Daemon create default config failed", err)
					return
				}
			}
			annoKey, err := cmd.Flags().GetString("annotation-key")
			if err != nil {
				logrus.WithField("daemon", "run").Error(err)
			} else {
				logrus.WithField("daemon", "run").Info("set annotation-key ", annoKey)
			}

			start := time.Now()
			signals := make(chan os.Signal, 2048)
			serviceChan := make(chan service.Service, 1)

			logrus.WithField("daemon", "run").Info("starting sichek daemon service")

			done := service.HandleSignals(cancel, signals, serviceChan)
			signal.Notify(signals, service.AllowedSignals...)
			componentConfig, err := config.LoadComponentConfig(cfgFile, specFile)
			if err != nil {
				logrus.WithField("daemon", "run").Errorf("create component config failed: %v", err)
				return
			}
			daemonService, err := service.NewService(ctx, cfg, componentConfig, annoKey)
			if err != nil {
				logrus.WithField("daemon", "run").Errorf("create daemon service failed: %v", err)
				return
			}
			serviceChan <- daemonService
			go daemonService.Run()

			if exist, _ := pkg_systemd.SystemctlExists(); exist {
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
