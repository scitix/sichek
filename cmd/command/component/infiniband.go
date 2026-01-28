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
	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewInfinibandCmd() *cobra.Command {
	var (
		specFile        string
		verbose         bool
		ignoredCheckers string
	)
	infinibandCmd := &cobra.Command{
		Use:     "infiniband",
		Aliases: []string{"i"},
		Short:   "Perform Infiniband HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)

			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				logrus.SetLevel(logrus.DebugLevel)
				defer func() {
					logrus.WithField("component", "infiniband").Info("Run infiniband Cmd context canceled")
					cancel()
				}()
			}

			specFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "infiniband").Errorf("using default specFile: %v", err)
			} else {
				logrus.WithField("daemon", "infiniband").Info("load specFile: " + specFile)
			}
			logrus.WithField("component", "infiniband").Info("ignore checkers: ", ignoredCheckers)
			var ignoredCheckersList []string
			if len(ignoredCheckers) > 0 {
				ignoredCheckersList = strings.Split(ignoredCheckers, ",")
			}

			component, err := infiniband.NewInfinibandComponent("", specFile, ignoredCheckersList)
			if err != nil {
				logrus.WithField("component", "infiniband").Error(err)
				return
			}
			logrus.WithField("component", "infiniband").Infof("Run Infiniband component check: %s", component.Name())
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	infinibandCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the Infiniband Spec")
	infinibandCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	infinibandCmd.Flags().StringVarP(&ignoredCheckers, "ignored-checkers", "i", "", "Ignored checkers")

	return infinibandCmd
}
