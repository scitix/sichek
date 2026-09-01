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

	"github.com/scitix/sichek/cmd/command/spec"
	"github.com/scitix/sichek/components/rdmaenv"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewRdmaEnvCmd() *cobra.Command {
	var (
		cfgFile string
		verbose bool
	)
	rdmaEnvCmd := &cobra.Command{
		Use:     "rdmaenv",
		Aliases: []string{"rdma"},
		Short:   "Passthrough rdma-env-pre exporter metrics into sichek",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			defer cancel()

			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			} else {
				logrus.SetLevel(logrus.DebugLevel)
			}

			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("component", "rdmaenv").Errorf("failed to load cfgFile: %v", err)
			} else {
				logrus.WithField("component", "rdmaenv").Info("load cfgFile: " + resolvedCfgFile)
			}

			component, err := rdmaenv.NewComponent(resolvedCfgFile, "")
			if err != nil {
				logrus.WithField("component", "rdmaenv").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	rdmaEnvCmd.Flags().StringVarP(&cfgFile, "cfg", "c", "", "Path to the user config file")
	rdmaEnvCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return rdmaEnvCmd
}
