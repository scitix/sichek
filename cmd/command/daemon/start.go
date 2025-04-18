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

// NewDaemonStartCmd creates and returns a subcommand instance for directly running a daemon process, configuring the basic attributes of the command.  
func NewDaemonStartCmd() *cobra.Command {
	daemonStartCmd := &cobra.Command{
		Use:   "start",
		Short: "Startup sichek daemon process in the background",
		Run: func(cmd *cobra.Command, args []string) {
			if exist, _ := pkgsystemd.SystemctlExists(); !exist {
				logrus.WithField("daemon", "start").Error("sichek start requires systemd")
				return
			}
			if !pkgutils.IsRoot() {
				logrus.WithField("daemon", "start").Error("sichek start requires root to run with systemd")
				return
			}
			if !systemd.DefaultBinExists() {
				logrus.WithField("daemon", "start").Errorf("sichek binary not found at %s", systemd.DefaultBinPath)
				return
			}
			if err := systemd.CreateDefaultEnvFile(); err != nil {
				logrus.WithField("daemon", "start").Error("failed to create systemd env file")
				return
			}
			systemdFileData := systemd.SichekService
			if err := os.WriteFile(systemd.DefaultUnitFile, []byte(systemdFileData), 0644); err != nil {
				logrus.WithField("daemon", "start").Error("failed to write systemd unit file")
				return
			}

			if err := systemd.LogrotateInit(); err != nil {
				logrus.WithField("daemon", "start").Error("failed to initialize logrotate for sichek log")
				return
			}

			if err := pkgsystemd.EnableSystemdService(consts.ServiceName); err != nil {
				logrus.WithField("daemon", "start").Error("failed to enable systemd unit 'sichek.service'")
				return
			}
			if err := pkgsystemd.RestartSystemdService(consts.ServiceName); err != nil {
				logrus.WithField("daemon", "start").Error("failed to restart systemd unit 'sichek.service'")
				return
			}

			logrus.WithField("daemon", "start").Info("start sichek service succeed")
		},
	}
	return daemonStartCmd
}
