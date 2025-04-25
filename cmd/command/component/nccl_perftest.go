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
	"os"

	"github.com/scitix/sichek/components/nccl/perftest"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewNcclPerftestCmd() *cobra.Command {

	ncclPerftestCmd := &cobra.Command{
		Use:   "nccl",
		Short: "Perform Nccl performance tests",
		Run: func(cmd *cobra.Command, args []string) {
			_, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				logrus.WithField("perftest", "nccl").Errorf("get to ge the verbose: %v", err)
			}
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("perftest", "nccl").Info("Run infiniband Cmd context canceled")
					cancel()
				}()
			}

			processCount, err := cmd.Flags().GetInt("process-count")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
			}
			enableNvls, err := cmd.Flags().GetBool("enable-nvls")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
			}
			expectedBandwidthGbps, err := cmd.Flags().GetFloat64("expect-bw")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
			}

			err = perftest.CheckNcclPerf(processCount, enableNvls, expectedBandwidthGbps)
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				os.Exit(-1)
			}
		},
	}

	ncclPerftestCmd.Flags().IntP("process-count", "c", 0, "Process count of test")
	ncclPerftestCmd.Flags().BoolP("enable-nvls", "e", false, "test with nvlinks")
	ncclPerftestCmd.Flags().Float64("expect-bw", 0, "Expected bandwidth in Gbps")
	ncclPerftestCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")

	return ncclPerftestCmd
}
