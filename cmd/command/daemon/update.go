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

	"github.com/scitix/sichek/consts"
	pkgsystemd "github.com/scitix/sichek/pkg/systemd"
	pkgutils "github.com/scitix/sichek/pkg/utils"
	"github.com/scitix/sichek/systemd"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewDaemonupdateCmd 创建并返回用于直接运行 daemon 进程的子命令实例，配置命令的基本属性
func NewDaemonUpdateCmd() *cobra.Command {
	daemonUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "updateup sichek daemon process in the background",
		Run: func(cmd *cobra.Command, args []string) {
			if exist, _ := pkgsystemd.SystemctlExists(); !exist {
				logrus.WithField("daemon", "update").Error("sichek update requires systemd")
				return
			}
			if !pkgutils.IsRoot() {
				logrus.WithField("daemon", "update").Error("sichek update requires root to run with systemd")
				return
			}
			if !systemd.DefaultBinExists() {
				logrus.WithField("daemon", "update").Errorf("sichek binary not found at %s", systemd.DefaultBinPath)
				return
			}
			if err := systemd.CreateDefaultEnvFile(); err != nil {
				logrus.WithField("daemon", "update").Error("failed to create systemd env file")
				return
			}
			systemdFileData := systemd.SichekService
			if err := os.WriteFile(systemd.DefaultUnitFile, []byte(systemdFileData), 0644); err != nil {
				logrus.WithField("daemon", "update").Error("failed to write systemd unit file")
				return
			}

			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("daemon", "update").Errorf("sichek update daemon serivce get config failed %v", err)
				return
			} else {
				logrus.WithField("daemon", "update").Infof("load config file %s...", cfgFile)
			}

			if err = pkgsystemd.RestartSystemdService(consts.ServiceName); err != nil {
				logrus.WithField("daemon", "update").Error("failed to restart systemd unit 'sichek.service'")
				return
			}

			logrus.WithField("daemon", "update").Info("update sichek service succeed")
		},
	}
	daemonUpdateCmd.Flags().StringP("cfg", "c", "", "Path to service config file")
	return daemonUpdateCmd
}
