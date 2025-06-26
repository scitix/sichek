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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/components/dmesg"
	"github.com/scitix/sichek/components/gpfs"
	"github.com/scitix/sichek/components/hang"
	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/components/pcie/topotest"
	"github.com/scitix/sichek/components/podlog"
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

	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Perform all components check",
		Long:  "Used to perform all configured related operations, with specific functions to be expanded",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.AllCmdTimeout)
			defer cancel()
			verbos, err := cmd.Flags().GetBool("verbos")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the verbose: %v", err)
			}
			eventonly, err := cmd.Flags().GetBool("eventonly")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the eventonly: %v", err)
			}
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
			}

			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "all").Error(err)
			} else {
				if cfgFile != "" {
					logrus.WithField("component", "all").Info("load cfgFile: " + cfgFile)
				} else {
					logrus.WithField("component", "all").Info("load default cfg...")
				}
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("component", "all").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("component", "all").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("component", "all").Info("load default specFile...")
				}
			}

			usedComponentStr, err := cmd.Flags().GetString("enable-components")
			if err != nil {
				logrus.WithField("component", "all").Error(err)
			} else {
				logrus.WithField("component", "all").Infof("enable components = %v", usedComponentStr)
			}
			var usedComponents []string
			if len(usedComponentStr) > 0 {
				usedComponents = strings.Split(usedComponentStr, ",")
			}

			ignoreComponentStr, err := cmd.Flags().GetString("ignore-components")
			if err != nil {
				logrus.WithField("component", "all").Error(err)
			} else {
				logrus.WithField("component", "all").Infof("ignore-components = %v", ignoreComponentStr)
			}
			var ignoredComponents []string
			if len(ignoreComponentStr) > 0 {
				ignoredComponents = strings.Split(ignoreComponentStr, ",")
			}

			ignoredCheckersStr, err := cmd.Flags().GetString("ignored-checkers")
			if err != nil {
				logrus.WithField("component", "all").Error(err)
			} else {
				logrus.WithField("component", "all").Infof("ignored-checkers = %v", ignoredCheckersStr)
			}
			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}
			checkResults := make([]*CheckResults, len(consts.DefaultComponents))
			var wg sync.WaitGroup
			for idx, componentName := range consts.DefaultComponents {
				if slices.Contains(ignoredComponents, componentName) {
					continue
				}
				if len(usedComponentStr) > 0 && !slices.Contains(usedComponents, componentName) {
					continue
				}
				wg.Add(1)
				go func(idx int, componentName string) {
					defer wg.Done()
					component, err := NewComponent(componentName, cfgFile, specFile, ignoredCheckers)
					if err != nil {
						logrus.WithField("component", componentName).Errorf("failed to create component: %v", err)
						return
					}
					checkResults[idx], _ = RunComponentCheck(ctx, component, cfgFile, specFile, ignoredCheckers, consts.AllCmdTimeout)
				}(idx, componentName)

			}
			wg.Wait()
			for _, checkResult := range checkResults {
				if checkResult == nil {
					continue
				}
				PrintCheckResults(!eventonly, checkResult)
			}
			//check topo
			if utils.IsNvidiaGPUExist() {
				res, err := topotest.CheckGPUTopology(specFile)
				if err != nil {
					logrus.WithField("component", "topo").Errorf("check topotest err: %v", err)
					return
				}
				passed := topotest.PrintInfo(res, !eventonly && verbos)
				ComponentStatuses[res.Item] = passed
			}
		},
	}

	allCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	allCmd.Flags().BoolP("eventonly", "e", false, "Print events output only")
	allCmd.Flags().StringP("spec", "s", "", "Path to the sichek specification file")
	allCmd.Flags().StringP("cfg", "c", "", "Path to the sichek configuration file")
	allCmd.Flags().StringP("enable-components", "E", "", "Enabled components, joined by ','")
	allCmd.Flags().StringP("ignore-components", "I", "podlog,hang", "Ignored components")
	allCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")

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
		return dmesg.NewComponent(cfgFile, specFile)
	case consts.ComponentNameHang:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing Hang HealthCheck")
		}
		_, err := nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
		if err != nil {
			return nil, fmt.Errorf("failed to Get Nvidia component, Bypassing HealthCheck")
		}
		return hang.NewComponent(cfgFile, specFile)
	case consts.ComponentNameNvidia:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing Nvidia GPU HealthCheck")
		}
		return nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
	case consts.ComponentNamePodLog:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing PodLog HealthCheck")
		}
		return podlog.NewComponent(cfgFile, specFile)
	default:
		return nil, fmt.Errorf("invalid component name: %s", componentName)
	}
}
