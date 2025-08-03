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
package command

import (
	"fmt"
	"os"

	"github.com/scitix/sichek/cmd/command/component"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command (sichek command) instance, configures basic usage information, and adds subcommands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sichek",
		Short: "Hardware health check tool",
		Long:  "A command - line tool for performing operations related to different hardware components",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			commandsRequireRoot := map[string]bool{
				"gpu":        true,
				"g":          true,
				"infiniband": true,
				"i":          true,
				"gpuevents":  true,
				"h":          true,
				"all":        true,
				"run":        true,
			}

			if commandsRequireRoot[cmd.Use] {
				root := utils.IsRoot()
				if !root {
					fmt.Printf("[ERROR] Command '%s' requires root privileges. Please run as root.\n", cmd.Use)
					os.Exit(-1)
				}
			}
			return nil
		},
	}

	rootCmd.AddCommand(component.NewCPUCmd())
	rootCmd.AddCommand(component.NewNvidiaCmd())
	rootCmd.AddCommand(component.NewInfinibandCmd())
	// rootCmd.AddCommand(component.NewEthernetCmd())
	rootCmd.AddCommand(component.NewGpfsCmd())
	rootCmd.AddCommand(component.NewPodLogCmd())
	rootCmd.AddCommand(component.NewDmesgCmd())
	rootCmd.AddCommand(component.NewGpuEventsCommand())
	rootCmd.AddCommand(component.NewMemoryCmd())
	rootCmd.AddCommand(component.NewAllCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewDaemonCmd())
	rootCmd.AddCommand(component.NewPcieTopoCmd())
	rootCmd.AddCommand(component.NewIBLinkCheckCmd())
	rootCmd.AddCommand(component.NewRoCEGidsCheckCmd())
	rootCmd.AddCommand(component.NewRoCEGidEqualCheckCmd())
	rootCmd.AddCommand(component.NewIBPerftestCmd())
	rootCmd.AddCommand(component.NewNcclPerftestCmd())
	rootCmd.AddCommand(component.NewRoCEPerftestCmd())
	rootCmd.AddCommand(component.NewMpiJobCmd())
	rootCmd.AddCommand(component.NewPytorchjobCmd())
	rootCmd.AddCommand(component.NewATNCCLTest1Cmd())
	rootCmd.AddCommand(component.NewATNCCLTest2Cmd())
	rootCmd.AddCommand(component.NewATLlama70bCmd())
	rootCmd.AddCommand(component.NewATLlama13bCmd())
	rootCmd.AddCommand(component.NewInstallCmd())
	rootCmd.AddCommand(component.NewUninstallCmd())
	rootCmd.AddCommand(component.NewNCCLDiagCmd())
	rootCmd.AddCommand(component.NewDiagCmd())
	rootCmd.AddCommand(component.NewDeployCmd())
	rootCmd.AddCommand(component.NewRunScriptsCmd())
	return rootCmd
}
