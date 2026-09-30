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

	"github.com/scitix/sichek/components/lldp"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewLldpCmd() *cobra.Command {
	lldpCmd := &cobra.Command{
		Use:     "lldp",
		Aliases: []string{"l"},
		Short:   "Show LLDP neighbors and local iface info",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			defer cancel()

			verbose, _ := cmd.Flags().GetBool("verbos")
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			}

			cfgFile, _ := cmd.Flags().GetString("cfg")
			specFile, _ := cmd.Flags().GetString("spec")

			c, err := lldp.NewComponent(cfgFile, specFile)
			if err != nil {
				logrus.WithField("component", "lldp").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, c, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	lldpCmd.Flags().StringP("cfg", "c", "", "Path to the lldp cfg")
	lldpCmd.Flags().StringP("spec", "s", "", "Unused for lldp; kept for symmetry with other components")
	lldpCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	return lldpCmd
}
