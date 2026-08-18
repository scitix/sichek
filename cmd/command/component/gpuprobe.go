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

	"github.com/scitix/sichek/components/gpuprobe"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewGpuProbeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gpu_probe",
		Aliases: []string{"gpuprobe"},
		Short:   "Perform active GPU compute self-test (idle-gated)",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			defer cancel()
			verbos, _ := cmd.Flags().GetBool("verbos")
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
			}
			cfgFile, _ := cmd.Flags().GetString("cfg")
			specFile, _ := cmd.Flags().GetString("spec")
			comp, err := gpuprobe.NewComponent(cfgFile, specFile)
			if err != nil {
				logrus.WithField("component", "gpuprobe").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, comp, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	cmd.Flags().StringP("cfg", "c", "", "Path to the gpuprobe cfg")
	cmd.Flags().StringP("spec", "s", "", "Path to the gpuprobe spec file")
	cmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	return cmd
}
