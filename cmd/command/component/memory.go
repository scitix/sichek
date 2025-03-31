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
package component

import (
	"context"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/memory"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewMemoryCmd创建并返回用于代表memory相关操作的子命令实例，配置命令的基本属性
func NewMemoryCmd() *cobra.Command {
	memoryCmd := &cobra.Command{
		Use:     "memory",
		Aliases: []string{"m"},
		Short:   "Perform Memory - related operations",
		Long:    "Used to perform specific Memory - related operations, with specific functions to be expanded",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), CmdTimeout)
			verbos, err := cmd.Flags().GetBool("verbos")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the verbose: %v", err)
			}
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("component", "memory").Info("Run memory Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "memory").Error(err)
				return
			} else {
				logrus.WithField("component", "memory").Info("load default cfg...")
			}
			component, err := memory.NewComponent(cfgFile)
			if err != nil {
				logrus.WithField("component", "memory").Errorf("create memory component failed: %v", err)
				return
			}

			result, err := component.HealthCheck(ctx)
			if err != nil {
				logrus.WithField("component", "memory").Errorf("analyze memory failed: %v", err)
				return
			}

			logrus.WithField("component", component.Name()).Infof("Analysis Result: %s\n", common.ToString(result))
			info, err := component.LastInfo(ctx)
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := PrintMemoryInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameMemory] = pass
			StatusMutex.Unlock()
		},
	}

	memoryCmd.Flags().StringP("cfg", "c", "", "Path to the memory Cfg")
	memoryCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return memoryCmd
}

func PrintMemoryInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	return true
}
