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
	"github.com/scitix/sichek/components/dmesg"
	"github.com/scitix/sichek/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewDmesgCmd() *cobra.Command {
	dmesgCmd := &cobra.Command{
		Use:     "dmesg",
		Aliases: []string{"m"},
		Short:   "Perform Dmesg check",
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
					logrus.WithField("component", "Dmesg").Info("Run Dmesg HealthCheck Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "Dmesg").Error(err)
				return
			} else {
				logrus.WithField("component", "Dmesg").Infof("load cfg file:%s", cfgFile)
			}
			cfg, err := config.LoadComponentConfig(cfgFile, "")
			if err != nil {
				logrus.WithField("component", "Dmesg").Errorf("create dmesg component failed: %v", err)
				return
			}
			component, err := dmesg.NewComponent(cfg)
			if err != nil {
				logrus.WithField("component", "Dmesg").Errorf("create dmesg component failed: %v", err)
				return
			}

			result, err := component.HealthCheck(ctx)
			if err != nil {
				logrus.WithField("component", "Dmesg").Errorf("analyze dmesg failed: %v", err)
				return
			}

			logrus.WithField("component", "Dmesg").Infof("Dmesg analysis result: \n%s", common.ToString(result))
			pass := PrintDmesgInfo(nil, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameDmesg] = pass
			StatusMutex.Unlock()
		},
	}

	dmesgCmd.Flags().StringP("cfg", "c", "", "Path to the Dmesg Cfg file")
	dmesgCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return dmesgCmd
}

func PrintDmesgInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	dmesgEvent := make(map[string]string)
	checkAllPassed := true
	checkerResults := result.Checkers
	for _, result := range checkerResults {
		switch result.Name {
		case "DmesgErrorChecker":
			if result.Status == consts.StatusAbnormal {
				checkAllPassed = false
				dmesgEvent["DmesgErrorChecker"] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		}
	}

	utils.PrintTitle("Dmesg", "-")
	if len(dmesgEvent) == 0 {
		fmt.Printf("%sNo Dmesg event detected%s\n", Green, Reset)
		return checkAllPassed
	}
	for n := range dmesgEvent {
		fmt.Printf("\tDetected %s Event\n", n)
	}
	return checkAllPassed
}
