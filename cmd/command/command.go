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
	"fmt"
	"os"

	"github.com/scitix/sichek/cmd/command/component"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/spf13/cobra"
)

// NewRootCmd创建并返回根命令（sichek命令）实例，配置基本使用信息以及添加子命令
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sichek",
		Short: "Hardware health check tool",
		Long:  "A command - line tool for performing operations related to different hardware components",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// 需要 root 权限的一级命令
			commandsRequireRoot := map[string]bool{
				"gpu":        true,
				"g":          true,
				"infiniband": true,
				"i":          true,
			}

			// 判断当前命令是否需要 root
			if commandsRequireRoot[cmd.Use] {
				root := utils.IsRoot()
				if !root {
					fmt.Printf("[ERROR] Command '%s' requires root privileges. Please run as root.\n", cmd.Use)
					os.Exit(-1)
				}
			}
			return nil
		},
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

	// add perftest subcommand
	perftestCmd := &cobra.Command{
		Use:   "perftest",
		Short: "Run performance tests",
	}
	rootCmd.AddCommand(perftestCmd)

	perftestCmd.AddCommand(component.NewIBPerftestCmd())

	return rootCmd
}
