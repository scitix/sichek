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
			var wg sync.WaitGroup
			for _, component_name := range config.DefaultComponents {
				var component common.Component
				var err error
				var printFunc func(common.Info, *common.Result, bool) bool
				switch component_name {
				case config.ComponentNameGpfs:
					component, err = gpfs.NewGpfsComponent("")
					printFunc = PrintGPFSInfo
				case config.ComponentNameCPU:
					component, err = cpu.NewComponent("")
					printFunc = PrintSystemInfo
				case config.ComponentNameInfiniband:
					component, err = infiniband.NewInfinibandComponent("")
					printFunc = PrintInfinibandInfo
				case config.ComponentNameDmesg:
					component, err = dmesg.NewComponent("")
					printFunc = PrintDmesgInfo
				case config.ComponentNameHang:
					component, err = hang.NewComponent("")
					printFunc = PrintHangInfo
				case config.ComponentNameNvidia:
					component, err = nvidia.NewComponent("", []string{})
					printFunc = PrintNvidiaInfo
				case config.ComponentNameNCCL:
					component, err = nccl.NewComponent("")
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

	return allCmd
}
