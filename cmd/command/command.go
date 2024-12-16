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
	"github.com/scitix/sichek/cmd/command/component"

	"github.com/spf13/cobra"
)

// NewRootCmd创建并返回根命令（sichek命令）实例，配置基本使用信息以及添加子命令
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sichek",
		Short: "Hardware health check tool",
		Long:  "A command - line tool for performing operations related to different hardware components",
	}

	// 添加子命令
	rootCmd.AddCommand(component.NewCPUCmd())
	rootCmd.AddCommand(component.NewNvidiaCmd())
	rootCmd.AddCommand(component.NewInfinibandCmd())
	rootCmd.AddCommand(component.NewEthernetCmd())
	rootCmd.AddCommand(component.NewGpfsCmd())
	rootCmd.AddCommand(component.NewNCCLCmd())
	rootCmd.AddCommand(component.NewDmesgCmd())
	rootCmd.AddCommand(component.NewHangCommand())
	rootCmd.AddCommand(component.NewMemoryCmd())
	rootCmd.AddCommand(component.NewAllCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewDaemonCmd())

	return rootCmd
}
