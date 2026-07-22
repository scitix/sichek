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

	"github.com/scitix/sichek/components/sysinfo"
	"github.com/scitix/sichek/components/sysinfo/collector"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewSysinfoCmd() *cobra.Command {
	var source string
	sysinfoCmd := &cobra.Command{
		Use:     "sysinfo",
		Aliases: []string{"si"},
		Short:   "Collect host/OS configuration via OSS KV scripts",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			defer cancel()
			verbos, _ := cmd.Flags().GetBool("verbos")
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
			}
			cfgFile, _ := cmd.Flags().GetString("cfg")

			// Single-source path.
			if source != "" {
				res, err := sysinfo.CollectOne(cfgFile, source)
				if err != nil {
					logrus.WithField("component", "sysinfo").Error(err)
					return
				}
				printSource(source, res, verbos)
				return
			}

			// All-sources path (mirrors other component subcommands).
			comp, err := sysinfo.NewComponent(cfgFile, "")
			if err != nil {
				logrus.WithField("component", "sysinfo").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, comp, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	sysinfoCmd.Flags().StringP("cfg", "c", "", "Path to the user config file")
	sysinfoCmd.Flags().StringVar(&source, "source", "", "Run only the named source")
	sysinfoCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output (dump all key=value)")
	return sysinfoCmd
}

func printSource(name string, res *collector.SourceResult, verbose bool) {
	fmt.Printf("sysinfo source %q: status=%s keys=%d source=%s\n", name, res.Status, res.KeyCount, res.Source)
	if res.Status != collector.StatusOK {
		fmt.Printf("  error: %s\n", res.Error)
		return
	}
	if verbose {
		for k, v := range res.Raw {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
}
