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

	"github.com/scitix/sichek/components/infiniband/perftest"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewIBPerftestCmd() *cobra.Command {

	ibPerftestCmd := &cobra.Command{
		Use:   "ib",
		Short: "Perform Infiniband performance tests",
		Run: func(cmd *cobra.Command, args []string) {
			_, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Errorf("get to ge the verbose: %v", err)
			}
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("perftest", "infiniband").Info("Run infiniband Cmd context canceled")
					cancel()
				}()
			}

			testType, err := cmd.Flags().GetString("test-type")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Error(err)
			}
			validTypes := map[string]bool{"ib_read_bw": true, "ib_write_bw": true}
			if !validTypes[testType] {
				logrus.WithField("perftest", "infiniband").Errorf("invalid testType: %s. Allowed values: ib_read_bw, ib_write_bw", testType)
				os.Exit(-1)
			}
			ibDevice, err := cmd.Flags().GetString("ib-dev")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Error(err)
			}
			size, err := cmd.Flags().GetInt("size")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Error(err)
			}
			duration, err := cmd.Flags().GetInt("duration")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Error(err)
			}
			numaAware, err := cmd.Flags().GetBool("numa-aware")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Errorf("get to ge the numaAware flag: %v", err)
			}
			expectedBandwidthGbps, err := cmd.Flags().GetFloat64("expect-bw")
			if err != nil {
				logrus.WithField("perftest", "infiniband").Error(err)
			}

			res, err := perftest.CheckNodeIBPerfHealth(testType, expectedBandwidthGbps, ibDevice, size, duration, numaAware, verbose)
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				os.Exit(-1)
			}
			passed := perftest.PrintInfo(res, verbose)
			ComponentStatuses[res.Item] = passed
		},
	}

	ibPerftestCmd.Flags().StringP("test-type", "t", "ib_read_bw", "IB test type (ib_read_bw, ib_write_bw), default is ib_read_bw")
	ibPerftestCmd.Flags().StringP("ib-dev", "d", "", "Use IB device <dev> (default all active devices found)")
	ibPerftestCmd.Flags().IntP("size", "s", 65536, "Size of message to exchange (default 65536)")
	ibPerftestCmd.Flags().IntP("duration", "D", 10, "Run test for a customized period of seconds (default 5 seconds)")
	ibPerftestCmd.Flags().BoolP("numa-aware", "n", false, "Run test with numa aware (default 'false`)")
	ibPerftestCmd.Flags().Float64("expect-bw", 0, "Expected bandwidth in Gbps")
	ibPerftestCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")

	return ibPerftestCmd
}
