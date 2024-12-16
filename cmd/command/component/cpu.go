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
	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/components/cpu/checker"
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/config"
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCPUCmd() *cobra.Command {
	cpuCmd := &cobra.Command{
		Use:     "cpu",
		Aliases: []string{"c"},
		Short:   "Perform CPU - related operations",
		Long:    "Used to perform specific CPU - related operations, with specific functions to be expanded",
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
					logrus.WithField("component", "cpu").Info("Run CPU HealthCheck Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "cpu").Error(err)
				return
			} else {
				logrus.WithField("component", "cpu").Info("load default cfg...")
			}

			component, err := cpu.NewComponent(cfgFile)
			if err != nil {
				logrus.WithField("component", "cpu").Errorf("create cpu component failed: %v", err)
				return
			}

			result, err := component.HealthCheck(ctx)
			if err != nil {
				logrus.WithField("component", "cpu").Errorf("analyze cpu failed: %v", err)
				return
			}

			logrus.WithField("component", component.Name()).Infof("Analysis Result: %s\n", common.ToString(result))
			info, err := component.LastInfo(ctx)
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := PrintSystemInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[config.ComponentNameCPU] = pass
			StatusMutex.Unlock()
		},
	}

	cpuCmd.Flags().StringP("cfg", "c", "", "Path to the cpu Cfg")
	cpuCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	return cpuCmd
}

func PrintSystemInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	cpuInfo, ok := info.(*collector.CPUOutput)
	if !ok {
		logrus.WithField("component", "cpu").Errorf("invalid data type, expected CPUOutput")
		return false
	}
	osPrint := fmt.Sprintf("OS: %s%s(%s)%s", Green, cpuInfo.HostInfo.OSVersion, cpuInfo.HostInfo.KernelVersion, Reset)
	modelNamePrint := fmt.Sprintf("ModelName: %s%s%s", Green, cpuInfo.CPUArchInfo.ModelName, Reset)
	uptimePrint := fmt.Sprintf("Uptime: %s%s%s", Green, cpuInfo.Uptime, Reset)

	nuamNodes := make([]string, 0, len(cpuInfo.CPUArchInfo.NumaNodeInfo))
	for _, node := range cpuInfo.CPUArchInfo.NumaNodeInfo {
		numaNode := fmt.Sprintf("NUMA node%d CPU(s): %s%s%s", node.ID, Green, node.CPUs, Reset)
		nuamNodes = append(nuamNodes, numaNode)
	}
	taskPrint := fmt.Sprintf("Tasks: %d, %s%d%s thr; %d running", cpuInfo.UsageInfo.SystemProcessesTotal, Green, cpuInfo.UsageInfo.TotalThreadCount, Reset, cpuInfo.UsageInfo.RunnableTaskCount)
	loadAvgPrint := fmt.Sprintf("Load average: %s%.2f %.2f %.2f%s", Green, cpuInfo.UsageInfo.CpuLoadAvg1m, cpuInfo.UsageInfo.CpuLoadAvg5m, cpuInfo.UsageInfo.CpuLoadAvg15m, Reset)
	var performanceModePrint string
	checkerResults := result.Checkers
	for _, result := range checkerResults {
		statusColor := Green
		if result.Status != commonCfg.StatusNormal {
			statusColor = Red
			checkAllPassed = false
		}
		switch result.Name {
		case checker.CPUPerfCheckerName:
			performanceModePrint = fmt.Sprintf("PerformanceMode: %s%s%s", statusColor, result.Curr, Reset)
		}
	}
	if summaryPrint {
		fmt.Printf("\nHostname: %s\n\n", cpuInfo.HostInfo.Hostname)
		utils.PrintTitle("System", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 40
		if err == nil {
			printInterval = termWidth / 3
		}
		fmt.Printf("%-*s%-*s\n", printInterval, osPrint, printInterval, modelNamePrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, uptimePrint, printInterval, nuamNodes[0], printInterval, taskPrint)
		if cpuInfo.CPUArchInfo.NumaNum > 1 {
			fmt.Printf("%-*s%-*s%-*s\n", printInterval, Green+""+Reset, printInterval, nuamNodes[1], printInterval, loadAvgPrint)
		} else {
			fmt.Printf("%-*s%-*s%-*s\n", printInterval, Green+""+Reset, printInterval, Green+""+Reset, printInterval, loadAvgPrint)
		}
		// TODO: more numa node
		fmt.Printf("%-*s%-*s\n", printInterval, Green+""+Reset, printInterval, performanceModePrint)
		fmt.Println()
	}
	return checkAllPassed
}
