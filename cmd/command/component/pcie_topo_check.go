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
	"os"

	"github.com/scitix/sichek/components/pcie/topotest"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewPcieTopoCmd() *cobra.Command {

	pcieTopoCmd := &cobra.Command{
		Use:   "topo",
		Short: "Perform pcie_topo check",
		Run: func(cmd *cobra.Command, args []string) {
			_, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				logrus.WithField("component", "topo").Errorf("get to ge the verbose: %v", err)
			}
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("component", "topo").Info("Run topo Cmd context canceled")
					cancel()
				}()
			}
			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("components", "topo").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("components", "topo").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("components", "topo").Info("load default specFile...")
				}
			}
			res, err := topotest.CheckGPUTopology(specFile)
			if err != nil {
				logrus.WithField("component", "topo").Errorf("check topotest err: %v", err)
				os.Exit(-1)
			}
			passed := topotest.PrintInfo(res, verbose)
			ComponentStatuses[res.Item] = passed
		},
	}

	pcieTopoCmd.Flags().StringP("spec", "s", "", "Path to the topo test specification file")
	pcieTopoCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")

	return pcieTopoCmd
}
