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
package command

import (
	"github.com/scitix/sichek/cmd/command/daemon"

	"github.com/spf13/cobra"
)

// NewDaemonCmd creates and returns a subcommand instance for running in daemon mode, configuring the basic attributes of the command.
func NewDaemonCmd() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:     "daemon",
		Aliases: []string{"d"},
		Short:   "Run in daemon mode",
		Long:    "Start the application in daemon mode for continuous monitoring or other background tasks",
	}

	daemonCmd.AddCommand(daemon.NewDaemonRunCmd())
	daemonCmd.AddCommand(daemon.NewDaemonStartCmd())
	daemonCmd.AddCommand(daemon.NewDaemonStopCmd())
	daemonCmd.AddCommand(daemon.NewDaemonUpdateCmd())
	return daemonCmd
}
