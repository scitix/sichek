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

	"github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/components/infiniband/perftest"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewRoCEPerftestCmd() *cobra.Command {

	ibPerftestCmd := &cobra.Command{
		Use:   "rocetest",
		Short: "Perform Infiniband performance tests",
		Run: func(cmd *cobra.Command, args []string) {
			_, cancel := context.WithTimeout(context.Background(), consts.IbPerfTestTimeout)
			passed := true

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				logrus.WithField("perftest", "roce").Errorf("get to ge the verbose: %v", err)
				passed = false
			}
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("perftest", "roce").Info("Run infiniband Cmd context canceled")
					cancel()
				}()
			}

			testType, err := cmd.Flags().GetString("test-type")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			validTypes := map[string]bool{"ib_read_bw": true, "ib_write_bw": true, "ib_read_lat": true, "ib_write_lat": true}
			if !validTypes[testType] {
				logrus.WithField("perftest", "roce").Errorf("invalid testType: %s. Allowed values: ib_read_bw, ib_write_bw, ib_read_lat, ib_write_lat", testType)
				passed = false
			}
			ibDevice, err := cmd.Flags().GetString("ib-dev")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			netDevice, err := cmd.Flags().GetString("net-dev")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			if ibDevice != "" && netDevice != "" {
				logrus.WithField("perftest", "roce").Errorf("only one of --ib-dev or --net-dev can be specified")
				passed = false
			}
			size, err := cmd.Flags().GetInt("size")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			duration, err := cmd.Flags().GetInt("duration")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			gid, err := cmd.Flags().GetInt("gid-index")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			qpNum, err := cmd.Flags().GetInt("qp")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			useGDR, err := cmd.Flags().GetBool("gdr")
			if err != nil {
				logrus.WithField("perftest", "roce").Errorf("get to ge the gdr flag: %v", err)
				passed = false
			}
			expectedBandwidthGbps, err := cmd.Flags().GetFloat64("expect-bw")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			expectedLatencyUs, err := cmd.Flags().GetFloat64("expect-lat")
			if err != nil {
				logrus.WithField("perftest", "roce").Error(err)
				passed = false
			}
			if expectedBandwidthGbps == 0 && expectedLatencyUs == 0 {
				specs, err := config.LoadSpec("/var/sichek/config/default_spec.yaml")
				if err != nil {
					logrus.WithField("perftest", "roce").Errorf("failed to load HCA spec config: %v", err)
					fmt.Println("No expected bandwidth or latency specified, using 0 Gbps and 0 us")
				} else {
					for _, spec := range specs.HcaSpec {
						expectedBandwidthGbps = spec.Perf.OneWayBW
						expectedLatencyUs = spec.Perf.AvgLatency
						fmt.Printf("Using %s expected bandwidth: %.2f Gbps and latency: %.2f us\n", spec.Hardware.VPD, spec.Perf.OneWayBW, spec.Perf.AvgLatency)
						break // Use the first spec, assuming all have the same perf values
					}
				}
			} else {
				fmt.Printf("Using provided expected bandwidth: %.2f Gbps and latency: %.2f us\n", expectedBandwidthGbps, expectedLatencyUs)
			}

			if passed {
				res, err := perftest.CheckRoCEPerfHealth(testType, expectedBandwidthGbps, expectedLatencyUs, ibDevice, netDevice, size, duration, gid, qpNum, useGDR, verbose)
				if err != nil {
					logrus.WithField("perftest", "roce").Error(err)
					passed = false
				}
				passed = perftest.PrintInfo(res, verbose)
			}
			ComponentStatuses[perftest.IBPerfTestName] = passed
		},
	}

	ibPerftestCmd.Flags().StringP("test-type", "t", "ib_write_bw", "IB test type (ib_read_bw, ib_write_bw)")
	ibPerftestCmd.Flags().StringP("ib-dev", "d", "", "Use IB device <dev1,dev2> (default all active devices found)")
	ibPerftestCmd.Flags().StringP("net-dev", "i", "", "Use RoCE net device <net1,net2> (default all active RoCE devices found)")
	ibPerftestCmd.Flags().IntP("size", "s", 65536, "Size of message to exchange")
	ibPerftestCmd.Flags().IntP("duration", "D", 5, "Run test for a customized period of seconds")
	ibPerftestCmd.Flags().BoolP("gdr", "g", false, "Run test with gdr")
	ibPerftestCmd.Flags().IntP("gid-index", "x", 3, "Test uses GID with GID index")
	ibPerftestCmd.Flags().IntP("qp", "q", 2, "Num of qps to use for the test")
	ibPerftestCmd.Flags().Float64("expect-bw", 0, "Expected bandwidth in Gbps")
	ibPerftestCmd.Flags().Float64("expect-lat", 0, "Expected latency in us")
	ibPerftestCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")

	return ibPerftestCmd
}
