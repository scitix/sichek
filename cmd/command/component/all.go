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
	"slices"
	"strings"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/components/dmesg"
	"github.com/scitix/sichek/components/gpfs"
	"github.com/scitix/sichek/components/hang"
	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/components/nccl"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	ComponentStatuses = make(map[string]bool) // Tracks pass/fail status for each component
	StatusMutex       sync.Mutex              // Ensures thread-safe updates
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
			ctx, cancel := context.WithTimeout(context.Background(), AllCmdTimeout)
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
				logrus.WithField("components", "all").Error(err)
			} else {
				if cfgFile != "" {
					logrus.WithField("components", "all").Info("load cfgFile: " + cfgFile)
				} else {
					logrus.WithField("components", "all").Info("load default cfg...")
				}
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("components", "all").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("components", "all").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("components", "all").Info("load default specFile...")
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
			var wg sync.WaitGroup
			for _, componentName := range consts.DefaultComponents {
				if slices.Contains(ignoredComponents, componentName) {
					continue
				}
				if len(usedComponentStr) > 0 && !slices.Contains(usedComponents, componentName) {
					continue
				}

				var component common.Component
				var err error
				var printFunc func(common.Info, *common.Result, bool) bool
				switch componentName {
				case consts.ComponentNameGpfs:
					component, err = gpfs.NewGpfsComponent(cfgFile)
					printFunc = PrintGPFSInfo
				case consts.ComponentNameCPU:
					component, err = cpu.NewComponent(cfgFile)
					printFunc = PrintSystemInfo
				case consts.ComponentNameInfiniband:
					component, err = infiniband.NewInfinibandComponent(cfgFile, specFile, ignoredCheckers)
					printFunc = PrintInfinibandInfo
				case consts.ComponentNameDmesg:
					component, err = dmesg.NewComponent(cfgFile)
					printFunc = PrintDmesgInfo
				case consts.ComponentNameHang:
					if !utils.IsNvidiaGPUExist() {
						logrus.Warn("Nvidia GPU is not Exist. Bypassing Hang HealthCheck")
						continue
					}
					_, err = nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
					if err != nil {
						logrus.WithField("component", "all").Errorf("Failed to Get Nvidia component, Bypassing HealthCheck")
						continue
					}
					component, err = hang.NewComponent(cfgFile)
					printFunc = PrintHangInfo
				case consts.ComponentNameNvidia:
					if !utils.IsNvidiaGPUExist() {
						logrus.Warn("Nvidia GPU is not Exist. Bypassing GPU HealthCheck")
						continue
					}
					component, err = nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
					printFunc = PrintNvidiaInfo
				case consts.ComponentNameNCCL:
					component, err = nccl.NewComponent(cfgFile)
					printFunc = PrintNCCLInfo
				default:
					logrus.WithField("component", "all").Errorf("invalid component_name: %s", componentName)
					continue
				}
				if err != nil {
					logrus.WithField("component", "all").Errorf("create component %s failed: %v", componentName, err)
					continue
				}
				result, err := common.RunHealthCheckWithTimeout(ctx, AllCmdTimeout, component.Name(), component.HealthCheck)
				if err != nil {
					logrus.WithField("component", "all").Errorf("analyze %s failed: %v", componentName, err)
					continue
				}
				info, err := component.LastInfo()
				if err != nil {
					logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
				}
				passed := printFunc(info, result, !eventonly)
				StatusMutex.Lock()
				ComponentStatuses[componentName] = passed
				StatusMutex.Unlock()
			}

			wg.Wait()
		},
	}

	allCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	allCmd.Flags().BoolP("eventonly", "e", false, "Print events output only")
	allCmd.Flags().StringP("spec", "s", "", "Path to the sichek specification file")
	allCmd.Flags().StringP("cfg", "c", "", "Path to the sichek configuration file")
	allCmd.Flags().StringP("enable-components", "E", "", "Enabled components, joined by ','")
	allCmd.Flags().StringP("ignore-components", "I", "", "Ignored components")
	allCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")

	return allCmd
}
