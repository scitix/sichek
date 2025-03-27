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
	"github.com/scitix/sichek/config"
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
	ctx, cancel := context.WithTimeout(context.Background(), AllCmdTimeout)
	defer cancel()

	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Perform all components check",
		Long:  "Used to perform all configured related operations, with specific functions to be expanded",
		Run: func(cmd *cobra.Command, args []string) {
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

			ignored_checkers_str, err := cmd.Flags().GetString("ignored-checkers")
			if err != nil {
				logrus.WithField("components", "all").Error(err)
			} else {
				logrus.WithField("components", "all").Info("ignore checkers", ignored_checkers_str)
			}
			ignoredCheckers := strings.Split(ignored_checkers_str, ",")
			cfg, err := config.LoadComponentConfig(cfgFile, specFile)
			if err != nil {
				logrus.WithField("component", "cpu").Errorf("create cpu component failed: %v", err)
				return
			}
			var wg sync.WaitGroup
			for _, component_name := range consts.DefaultComponents {
				var component common.Component
				var err error
				var printFunc func(common.Info, *common.Result, bool) bool
				switch component_name {
				case consts.ComponentNameGpfs:
					component, err = gpfs.NewGpfsComponent(cfg)
					printFunc = PrintGPFSInfo
				case consts.ComponentNameCPU:
					component, err = cpu.NewComponent(cfg)
					printFunc = PrintSystemInfo
				case consts.ComponentNameInfiniband:
					component, err = infiniband.NewInfinibandComponent(cfg, ignoredCheckers)
					printFunc = PrintInfinibandInfo
				case consts.ComponentNameDmesg:
					component, err = dmesg.NewComponent(cfg)
					printFunc = PrintDmesgInfo
				case consts.ComponentNameHang:
					if !utils.IsNvidiaGPUExist() {
						logrus.Warn("Nvidia GPU is not Exist. Bypassing Hang HealthCheck")
						continue
					}
					component, err = hang.NewComponent(cfg)
					printFunc = PrintHangInfo
				case consts.ComponentNameNvidia:
					if !utils.IsNvidiaGPUExist() {
						logrus.Warn("Nvidia GPU is not Exist. Bypassing GPU HealthCheck")
						continue
					}
					component, err = nvidia.NewComponent(cfg, ignoredCheckers)
					printFunc = PrintNvidiaInfo
				case consts.ComponentNameNCCL:
					component, err = nccl.NewComponent(cfg)
					printFunc = PrintNCCLInfo
				default:
					logrus.WithField("component", "all").Errorf("invalid component_name: %s", component_name)
					continue
				}
				if err != nil {
					logrus.WithField("component", "all").Errorf("create component %s failed: %v", component_name, err)
					continue
				}
				result, err := component.HealthCheck(ctx)
				if err != nil {
					logrus.WithField("component", "all").Errorf("analyze %s failed: %v", component_name, err)
					continue
				}
				info, err := component.LastInfo(ctx)
				if err != nil {
					logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
				}
				passed := printFunc(info, result, !eventonly)
				StatusMutex.Lock()
				ComponentStatuses[component_name] = passed
				StatusMutex.Unlock()
			}

			wg.Wait()
		},
	}

	allCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	allCmd.Flags().BoolP("eventonly", "e", false, "Print events output only")
	allCmd.Flags().StringP("spec", "s", "", "Path to the sichek specification file")
	allCmd.Flags().StringP("cfg", "c", "", "Path to the sichek configuration file")
	allCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")

	return allCmd
}
