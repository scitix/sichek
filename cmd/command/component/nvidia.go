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
	"strings"

	"github.com/scitix/sichek/cmd/command/spec"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewNvidiaCmd creates and returns a subcommand instance for representing gpu-related operations, configuring the basic attributes of the command.
func NewNvidiaCmd() *cobra.Command {

	NvidaCmd := &cobra.Command{
		Use:     "gpu",
		Aliases: []string{"g"},
		Short:   "Perform nvidia GPU HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			if !utils.IsNvidiaGPUExist() {
				logrus.Warn("nvidia GPU is not Exist. Bypassing GPU HealthCheck")
				logrus.Exit(0)
			}
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
					logrus.WithField("component", "nvidia").Info(fmt.Printf("Run NVIDIA HealthCheck Cmd context canceled"))
					cancel()
				}()
			}
			cfgFile, _ := cmd.Flags().GetString("cfg")
			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("daemon", "nvidia").Errorf("failed to load cfgFile: %v", err)
			} else {
				logrus.WithField("daemon", "nvidia").Info("load cfgFile: " + resolvedCfgFile)
			}
			specFile, _ := cmd.Flags().GetString("spec")
			resolvedSpecFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "nvidia").Errorf("failed to load specFile: %v", err)
			} else {
				logrus.WithField("daemon", "nvidia").Info("load specFile: " + resolvedSpecFile)
			}
			ignoredCheckersStr, err := cmd.Flags().GetString("ignored-checkers")
			if err != nil {
				logrus.WithField("component", "nvidia").Error(err)
			} else {
				logrus.WithField("component", "nvidia").Info("ignore checkers", ignoredCheckersStr)
			}
			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}
			component, err := nvidia.NewComponent(resolvedCfgFile, resolvedSpecFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "nvidia").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	NvidaCmd.Flags().StringP("cfg", "c", "", "Path to the user config file")
	NvidaCmd.Flags().StringP("spec", "s", "", "Path to the nvidia specification file")
	NvidaCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	NvidaCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")

	return NvidaCmd
}
