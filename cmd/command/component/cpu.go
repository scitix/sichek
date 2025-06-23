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

	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/consts"

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
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
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
			component, err := cpu.NewComponent(cfgFile, specFile)
			if err != nil {
				logrus.WithField("component", "cpu").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, cfgFile, "", nil, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)

		},
	}

	cpuCmd.Flags().StringP("cfg", "c", "", "Path to the cpu Cfg")
	cpuCmd.Flags().StringP("spec", "s", "", "Path to the cpu specification file")
	cpuCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	return cpuCmd
}
