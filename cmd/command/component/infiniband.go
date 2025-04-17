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

	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewInfinibandCmd() *cobra.Command {
	infinibandCmd := &cobra.Command{
		Use:     "infiniband",
		Aliases: []string{"i"},
		Short:   "Perform Infiniband check - related operations",
		Long:    "Used to perform specific Infiniband - related operations, with specific functions to be expanded",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the verbose: %v", err)
			}
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("component", "infiniband").Info("Run infiniband Cmd context canceled")
					cancel()
				}()
			}

			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "infiniband").Error(err)
				return
			} else {
				logrus.WithField("component", "infiniband").Info("load default cfg...")
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("components", "infiniband").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("components", "infiniband").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("components", "infiniband").Info("load default specFile...")
				}
			}

			ignoredCheckersStr, err := cmd.Flags().GetString("ignored-checkers")
			if err != nil {
				logrus.WithField("components", "infiniband").Error(err)
			} else {
				logrus.WithField("components", "infiniband").Info("ignore checkers", ignoredCheckersStr)
			}
			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}
			component, err := infiniband.NewInfinibandComponent(cfgFile, specFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "infiniband").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, cfgFile, specFile, ignoredCheckers, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	infinibandCmd.Flags().StringP("cfg", "c", "", "Path to the Infinibnad Cfg")
	infinibandCmd.Flags().StringP("spec", "s", "", "Path to the Infinibnad Spec")
	infinibandCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	infinibandCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")
	return infinibandCmd
}
