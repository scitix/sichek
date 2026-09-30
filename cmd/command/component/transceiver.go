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
	"github.com/scitix/sichek/components/transceiver"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewTransceiverCmd() *cobra.Command {
	var (
		cfgFile            string
		specFile           string
		ignoredCheckersStr string
		verbose            bool
	)
	transceiverCmd := &cobra.Command{
		Use:     "transceiver",
		Aliases: []string{"tr"},
		Short:   "Perform Transceiver HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			} else {
				logrus.SetLevel(logrus.DebugLevel)
			}

			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("daemon", "transceiver").Errorf("failed to load cfgFile: %v", err)
			} else {
				logrus.WithField("daemon", "transceiver").Info("load cfgFile: " + resolvedCfgFile)
			}
			resolvedSpecFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "transceiver").Errorf("failed to load specFile: %v", err)
			} else {
				logrus.WithField("daemon", "transceiver").Info("load specFile: " + resolvedSpecFile)
			}

			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}

			component, err := transceiver.NewComponent(resolvedCfgFile, resolvedSpecFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "transceiver").Error(err)
				return
			}
			// Start the health-check budget only now that config, spec and the
			// component are resolved. Those steps fetch from OSS, and on a node with
			// no route to it each attempt blocks until its own timeout — roughly 14s
			// in total when measured. Counting that against the collection budget
			// left the collector ~16s of its 30s and made it time out with no data.
			ctx, cancel := context.WithTimeout(context.Background(), consts.TransceiverCmdTimeout)
			if verbose {
				defer func() {
					logrus.WithField("component", "transceiver").Info("Run transceiver Cmd context canceled")
					cancel()
				}()
			} else {
				defer cancel()
			}

			logrus.WithField("component", "transceiver").Infof("Run Transceiver component check: %s", component.Name())
			result, err := RunComponentCheck(ctx, component, consts.TransceiverCmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	transceiverCmd.Flags().StringVarP(&cfgFile, "cfg", "c", "", "Path to the user config file")
	transceiverCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the transceiver specification file")
	transceiverCmd.Flags().StringVarP(&ignoredCheckersStr, "ignored-checkers", "i", "", "Ignored checkers")
	transceiverCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return transceiverCmd
}
