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

// NewDaemonCmd 创建并返回用于以daemon状态运行的子命令实例，配置命令的基本属性
func NewDaemonCmd() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:     "daemon",
		Aliases: []string{"d"},
		Short:   "Run in daemon mode",
		Long:    "Start the application in daemon mode for continuous monitoring or other background tasks",
		// 此处暂不添加具体的Run逻辑，只定义命令结构
	}

	// 添加子命令
	daemonCmd.AddCommand(daemon.NewDaemonRunCmd())
	daemonCmd.AddCommand(daemon.NewDaemonStartCmd())
	daemonCmd.AddCommand(daemon.NewDaemonStopCmd())
	daemonCmd.AddCommand(daemon.NewDaemonUpdateCmd())
	return daemonCmd
}
