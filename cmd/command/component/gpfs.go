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
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpfs"
	"github.com/scitix/sichek/components/gpfs/collector"
	"github.com/scitix/sichek/components/gpfs/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewGpfsCmd 创建并返回用于代表Gpfs相关操作的子命令实例，配置命令的基本属性
func NewGpfsCmd() *cobra.Command {
	gpfsCmd := &cobra.Command{
		Use:     "gpfs",
		Aliases: []string{"f"},
		Short:   "Perform Gpfs - related operations",
		Long:    "Used to perform specific Gpfs - related operations, with specific functions to be expanded",
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
					logrus.WithField("component", "Gpfs").Info("Run Gpfs Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "Gpfs").Error(err)
				return
			} else {
				logrus.WithField("component", "Gpfs").Info("load default cfg...")
			}
			component, err := gpfs.NewGpfsComponent(cfgFile)
			if err != nil {
				logrus.WithField("component", "Gpfs").Errorf("create gpfs component failed: %v", err)
				return
			}

			result, err := component.HealthCheck(ctx)
			if err != nil {
				logrus.WithField("component", "Gpfs").Errorf("analyze gpfs failed: %v", err)
				return
			}

			logrus.WithField("component", "Gpfs").Infof("Gpfs analysis result: %s\n", common.ToString(result))
			info, err := component.LastInfo(ctx)
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := PrintGPFSInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameGpfs] = pass
			StatusMutex.Unlock()
		},
	}

	gpfsCmd.Flags().StringP("cfg", "c", "", "Path to the Gpfs Cfg")
	gpfsCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return gpfsCmd
}

func PrintGPFSInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	_, ok := info.(*collector.GPFSInfo)
	if !ok {
		logrus.WithField("component", "Gpfs").Errorf("invalid data type, expected GPFSInfo")
		return false
	}
	var mountPrint string
	gpfsEvent := make(map[string]string)

	checkerResults := result.Checkers
	for _, result := range checkerResults {
		statusColor := Green
		if result.Status != consts.StatusNormal {
			statusColor = Red
			checkAllPassed = false
			gpfsEvent[result.Name] = fmt.Sprintf("Event: %s%s%s", Red, result.ErrorName, Reset)
		}

		switch result.Name {
		case config.FilesystemUnmountCheckerName:
			mountPrint = fmt.Sprintf("GPFS: %sMounted%s", statusColor, Reset)
		}
	}

	utils.PrintTitle("GPFS", "-")
	fmt.Printf("%s\n", mountPrint)
	for _, v := range gpfsEvent {
		fmt.Printf("\t%s\n", v)
	}
	return checkAllPassed
}
