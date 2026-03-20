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
	"github.com/scitix/sichek/components/ethernet"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewEthernetCmd() *cobra.Command {
	var (
		cfgFile            string
		specFile           string
		ignoredCheckersStr string
		verbose            bool
	)
	ethernetCmd := &cobra.Command{
		Use:     "ethernet",
		Aliases: []string{"e"},
		Short:   "Perform Ethernet HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)

			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				logrus.SetLevel(logrus.DebugLevel)
				defer func() {
					logrus.WithField("component", "ethernet").Info("Run ethernet Cmd context canceled")
					cancel()
				}()
			}

			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("daemon", "ethernet").Errorf("failed to load cfgFile: %v", err)
			} else {
				logrus.WithField("daemon", "ethernet").Info("load cfgFile: " + resolvedCfgFile)
			}
			resolvedSpecFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "ethernet").Errorf("failed to load specFile: %v", err)
			} else {
				logrus.WithField("daemon", "ethernet").Info("load specFile: " + resolvedSpecFile)
			}

			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}

			component, err := ethernet.NewEthernetComponent(resolvedCfgFile, resolvedSpecFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "ethernet").Error(err)
				return
			}
			logrus.WithField("component", "ethernet").Infof("Run Ethernet component check: %s", component.Name())
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	ethernetCmd.Flags().StringVarP(&cfgFile, "cfg", "c", "", "Path to the user config file")
	ethernetCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the Ethernet specification file")
	ethernetCmd.Flags().StringVarP(&ignoredCheckersStr, "ignored-checkers", "i", "", "Ignored checkers")
	ethernetCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return ethernetCmd
}
