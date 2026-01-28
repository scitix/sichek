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
	"slices"
	"strings"
	"sync"

	"github.com/scitix/sichek/cmd/command/spec"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/components/dmesg"
	"github.com/scitix/sichek/components/gpfs"
	gpuevents "github.com/scitix/sichek/components/gpuevents"
	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/components/pcie/topotest"
	"github.com/scitix/sichek/components/podlog"
	"github.com/scitix/sichek/components/syslog"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewAllCmd creates a new cobra.Command for performing health checks on all components.
// It sets up the command with a context that times out after AllCmdTimeout, and defines the
// command's usage, short description, and long description. The command iterates over
// a list of default components, performs health checks on each, and prints the results.
// Flags:
// - verbos: Enable verbose output (default: false)
// - eventonly: Print events output only (default: false)
func NewAllCmd() *cobra.Command {
	var (
		specFile         string
		enableComponents string
		ignoreComponents string
		ignoredCheckers  string
		verbos           bool
		eventonly        bool
	)
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Perform all components check",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.AllCmdTimeout)
			defer cancel()

			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
			}
			specFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "all").Errorf("using default specFile: %v", err)
			} else {
				logrus.WithField("daemon", "all").Info("load specFile: " + specFile)
			}

			logrus.WithField("component", "all").Infof("ignored-checkers = %v", ignoredCheckers)
			var ignoredCheckersList []string
			if len(ignoredCheckers) > 0 {
				ignoredCheckersList = strings.Split(ignoredCheckers, ",")
			}

			componentsToCheck := DetermineComponentsToCheck(enableComponents, ignoreComponents, "", "all")
			checkResults := make([]*CheckResults, len(componentsToCheck))
			var wg sync.WaitGroup
			for idx, componentName := range componentsToCheck {
				if componentName == consts.ComponentNameInfiniband && !utils.IsInfinibandExist() {
					continue
				}
				if !slices.Contains(consts.DefaultComponents, componentName) {
					continue
				}
				wg.Add(1)
				go func(idx int, componentName string) {
					defer wg.Done()
					component, err := NewComponent(componentName, "", specFile, ignoredCheckersList)
					if err != nil {
						logrus.WithField("component", componentName).Errorf("failed to create component: %v", err)
						return
					}
					checkResults[idx], _ = RunComponentCheck(ctx, component, consts.AllCmdTimeout)
				}(idx, componentName)

			}
			wg.Wait()
			for _, checkResult := range checkResults {
				if checkResult == nil {
					continue
				}
				PrintCheckResults(!eventonly, checkResult)
			}

			if utils.IsNvidiaGPUExist() {

				// check nccl perf test
				if slices.Contains(componentsToCheck, "nccltest") {
					ncclCmd := NewNcclPerftestCmd()
					args := []string{"--begin", "2g", "--end", "2g"}
					ncclCmd.SetArgs(args)
					fmt.Printf("Running NCCL performance test with args: %v\n", args)
					if err := ncclCmd.Execute(); err != nil {
						fmt.Printf("failed to run NCCL test: %v\n", err)
					}
				}
				// check pcie topology
				if slices.Contains(componentsToCheck, "pcie_topo") {
					res, err := topotest.CheckGPUTopology(specFile)
					if err != nil {
						logrus.WithField("component", "pcie_topo").Errorf("check pcie_topo err: %v", err)
						return
					}
					passed := topotest.PrintInfo(res, !eventonly && verbos)
					ComponentStatuses[res.Item] = passed
				}
			}
		},
	}

	allCmd.Flags().BoolVarP(&verbos, "verbos", "v", false, "Enable verbose output")
	allCmd.Flags().BoolVarP(&eventonly, "eventonly", "e", false, "Print events output only")
	allCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the sichek specification file")
	allCmd.Flags().StringVarP(&enableComponents, "enable-components", "E", "", "Enabled components, joined by ','")
	allCmd.Flags().StringVarP(&ignoreComponents, "ignore-components", "I", "podlog,gpuevents,syslog", "Ignored components")
	allCmd.Flags().StringVarP(&ignoredCheckers, "ignored-checkers", "i", "", "Ignored checkers")

	return allCmd
}

func NewComponent(componentName string, cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	switch componentName {
	case consts.ComponentNameGpfs:
		return gpfs.NewGpfsComponent(cfgFile, specFile)
	case consts.ComponentNameCPU:
		return cpu.NewComponent(cfgFile, specFile)
	case consts.ComponentNameInfiniband:
		return infiniband.NewInfinibandComponent(cfgFile, specFile, ignoredCheckers)
	case consts.ComponentNameDmesg:
		// if skipPercent is -1, use the value from the config file (default: 100)
		return dmesg.NewComponent(cfgFile, specFile, -1)
	case consts.ComponentNameGpuEvents:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing GpuEvents HealthCheck")
		}
		_, err := nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
		if err != nil {
			return nil, fmt.Errorf("failed to Get Nvidia component, Bypassing HealthCheck")
		}
		return gpuevents.NewComponent(cfgFile, specFile)
	case consts.ComponentNameNvidia:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing Nvidia GPU HealthCheck")
		}
		return nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
	case consts.ComponentNamePodlog:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing PodLog HealthCheck")
		}
		// if skipPercent is -1, use the value from the config file (default: 100)
		return podlog.NewComponent(cfgFile, specFile, true, -1) // default to only check running pods
	case consts.ComponentNameSyslog:
		// if skipPercent is -1, use the value from the config file
		return syslog.NewComponent(cfgFile, "", -1)
	default:
		return nil, fmt.Errorf("invalid component name: %s", componentName)
	}
}
