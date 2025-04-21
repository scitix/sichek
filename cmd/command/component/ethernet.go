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

	"github.com/scitix/sichek/components/ethernet"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewEthernetCmd() *cobra.Command {
	ethernetCmd := &cobra.Command{
		Use:     "ethernet",
		Aliases: []string{"e"},
		Short:   "Perform ethernet - related operations",
		Long:    "Used to perform specific ethernet - related operations, with specific functions to be expanded",
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
					logrus.WithField("component", "ethernet").Info("Run Ethernet HealthCheck Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "ethernet").Error(err)
			} else {
				logrus.WithField("component", "ethernet").Info("load default cfg...")
			}
			component, err := ethernet.NewEthernetComponent(cfgFile, "")
			if err != nil {
				logrus.WithField("component", "ethernet").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, cfgFile, "", nil, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	ethernetCmd.Flags().StringP("cfg", "c", "", "Path to the Infinibnad Cfg")
	ethernetCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return ethernetCmd
}
