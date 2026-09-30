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

	"github.com/scitix/sichek/cmd/command/spec"
	"github.com/scitix/sichek/components/ovs"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewOVSCmd() *cobra.Command {
	var (
		cfgFile            string
		specFile           string
		ignoredCheckersStr string
		verbose            bool
	)
	ovsCmd := &cobra.Command{
		Use:   "ovs",
		Short: "Perform OVS (DOCA Open vSwitch) HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				logrus.SetLevel(logrus.DebugLevel)
				defer cancel()
			}
			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("daemon", "ovs").Errorf("failed to load cfgFile: %v", err)
			}
			resolvedSpecFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "ovs").Errorf("failed to load specFile: %v", err)
			}
			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}
			component, err := ovs.NewComponent(resolvedCfgFile, resolvedSpecFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "ovs").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	ovsCmd.Flags().StringVarP(&cfgFile, "cfg", "c", "", "Path to the user config file")
	ovsCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the ovs specification file")
	ovsCmd.Flags().StringVarP(&ignoredCheckersStr, "ignored-checkers", "i", "", "Ignored checkers")
	ovsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	return ovsCmd
}
