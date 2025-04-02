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
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/scitix/sichek/consts"
	pkgsystemd "github.com/scitix/sichek/pkg/systemd"
	"github.com/scitix/sichek/pkg/utils"
)

// NewDaemonStopCmd 创建并返回用于停止daemon 进程的子命令实例，配置命令的基本属性
func NewDaemonStopCmd() *cobra.Command {

	daemonStopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop sichek daemon process",
		Run: func(cmd *cobra.Command, args []string) {
			if !utils.IsRoot() {
				logrus.WithField("daemon", "stop").Errorf("current user is not root")
				os.Exit(1)
			}
			if exist, _ := pkgsystemd.SystemctlExists(); !exist {
				logrus.WithField("daemon", "stop").Errorf("systemd not exist")
				os.Exit(1)
			}

			active, err := pkgsystemd.IsActive("sichek.service")
			if err != nil {
				logrus.WithField("daemon", "stop").Error(err)
				os.Exit(1)
			}
			if !active {
				logrus.WithField("daemon", "stop").Info("sichek is not running")
				return
			}

			err = pkgsystemd.StopSystemdService(consts.ServiceName)
			if err != nil {
				logrus.WithField("daemon", "stop").Error(err)
				return
			}
			err = pkgsystemd.DisableSystemdService(consts.ServiceName)
			if err != nil {
				logrus.WithField("daemon", "stop").Error(err)
				return
			}

			logrus.WithField("daemon", "stop").Info("Daemon service stop succeed.")
		},
	}
	return daemonStopCmd
}
