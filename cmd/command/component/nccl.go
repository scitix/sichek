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
	"github.com/scitix/sichek/components/nccl"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewNCCLCmd() *cobra.Command {
	ncclCmd := &cobra.Command{
		Use:     "nccl",
		Aliases: []string{"nc"},
		Short:   "Perform NCCL check",
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
					logrus.WithField("component", "NCCL").Info("Run NCCL Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "NCCL").Error(err)
				return
			} else {
				logrus.WithField("component", "NCCL").Infof("load cfg file:%s", cfgFile)
			}

			component, err := nccl.NewComponent(cfgFile)
			if err != nil {
				logrus.WithField("component", "NCCL").Errorf("create nccl component failed: %v", err)
				return
			}

			result, err := common.RunHealthCheckWithTimeout(ctx, CmdTimeout, component.Name(), component.HealthCheck)
			if err != nil {
				logrus.WithField("component", "NCCL").Errorf("analyze nccl failed: %v", err)
				return
			}

			logrus.WithField("component", "NCCL").Infof("NCCL analysis result: \n%s", common.ToString(result))
			pass := PrintNCCLInfo(nil, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameNCCL] = pass
			StatusMutex.Unlock()
		},
	}

	ncclCmd.Flags().StringP("cfg", "c", "", "Path to the NCCL Cfg file")
	ncclCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return ncclCmd
}

func PrintNCCLInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	ncclEvents := make(map[string]string)

	checkerResults := result.Checkers
	for _, result := range checkerResults {
		switch result.Name {
		case "NCCLTimeoutChecker":
			if result.Status == consts.StatusAbnormal {
				ncclEvents["NCCLTimeoutChecker"] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		}
	}
	utils.PrintTitle("NCCL Error", "-")
	if len(ncclEvents) == 0 {
		fmt.Printf("%sNo NCCL event detected%s\n", Green, Reset)
		return true
	}
	for _, v := range ncclEvents {
		fmt.Printf("\t%s\n", v)
	}
	return false
}
