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

	"github.com/scitix/sichek/components/podlog"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewPodLogCmd() *cobra.Command {
	ncclCmd := &cobra.Command{
		Use:     "nccl",
		Aliases: []string{"nc"},
		Short:   "Perform NCCL check",
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
					logrus.WithField("component", "NCCL").Info("Run NCCL Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "NCCL").Error(err)
				return
			} else {
				logrus.WithField("component", "NCCL").Infof("load cfg file:%s", cfgFile)
			}
			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("components", "NCCL").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("components", "NCCL").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("components", "NCCL").Info("load default specFile...")
				}
			}
			component, err := podlog.NewComponent(cfgFile, specFile)
			if err != nil {
				logrus.WithField("component", "NCCL").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, cfgFile, "", nil, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	ncclCmd.Flags().StringP("cfg", "c", "", "Path to the NCCL Cfg file")
	ncclCmd.Flags().StringP("spec", "s", "", "Path to the NCCL specification file")
	ncclCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return ncclCmd
}
