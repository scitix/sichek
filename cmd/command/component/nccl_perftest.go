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
	"errors"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/perftest"
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

			numGpus, err := cmd.Flags().GetInt("num-gpus")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			nvmlInst := nvml.New()
			if ret := nvmlInst.Init(); !errors.Is(ret, nvml.SUCCESS) {
				logrus.WithField("perftest", "nccl").Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
				return
			}
			defer nvmlInst.Shutdown()
			deviceCount, ret := nvmlInst.DeviceGetCount()
			if !errors.Is(ret, nvml.SUCCESS) {
				logrus.WithField("perftest", "nccl").Errorf("failed to get device count: %s", nvml.ErrorString(ret))
				return
			}
			if numGpus > deviceCount {
				logrus.WithField("perftest", "nccl").Warnf("num-gpus %d is greater than available GPUs %d, setting to %d", numGpus, deviceCount, deviceCount)
				numGpus = deviceCount
			}
			gpulist, err := cmd.Flags().GetString("gpulist")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			if gpulist != "" {
				gpus := strings.Split(gpulist, ",")
				specifiedNumGpus := len(gpus)
				if numGpus > specifiedNumGpus {
					logrus.WithField("perftest", "nccl").Warnf("num-gpus %d is greater than specified GPU list length %d, setting to %d", numGpus, specifiedNumGpus, specifiedNumGpus)
					numGpus = specifiedNumGpus
				}
			}
			beginBuffer, err := cmd.Flags().GetString("begin")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			endBuffer, err := cmd.Flags().GetString("end")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			disableNvls, err := cmd.Flags().GetBool("disable-nvls")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			scale, err := cmd.Flags().GetBool("scale-gpus")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			expectedBandwidthGbps, err := cmd.Flags().GetFloat64("expect-bw")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}

			var res *common.Result
			result := 0
			if scale {
				for g := 2; g <= numGpus; g++ {
					res, err = perftest.CheckNcclPerf(g, gpulist, beginBuffer, endBuffer, disableNvls, expectedBandwidthGbps)
					if err != nil {
						logrus.WithField("perftest", "nccl").Error(err)
						result = -1
					}
				}
			} else {
				res, err = perftest.CheckNcclPerf(numGpus, gpulist, beginBuffer, endBuffer, disableNvls, expectedBandwidthGbps)
				if err != nil {
					logrus.WithField("perftest", "nccl").Error(err)
					result = -1
				}
			}
			if result == 0 {
				passed := perftest.PrintNcclPerfInfo(res)
				ComponentStatuses[res.Item] = passed
			} else {
				ComponentStatuses["NcclPerf"] = false
			}
		},
	}

	ncclPerftestCmd.Flags().IntP("num-gpus", "n", 8, "Max GPU numbers to test")
	ncclPerftestCmd.Flags().StringP("gpulist", "g", "", "specific GPU list to test, e.g. 0,1,2,3")
	ncclPerftestCmd.Flags().StringP("begin", "b", "", "begin buffer")
	ncclPerftestCmd.Flags().StringP("end", "e", "", "end buffer")
	ncclPerftestCmd.Flags().Bool("scale-gpus", false, "Run NCCL test scaling GPU count from 2 to n")
	ncclPerftestCmd.Flags().BoolP("disable-nvls", "d", false, "test without nvlinks")
	ncclPerftestCmd.Flags().Float64("expect-bw", 0, "Expected bandwidth in Gbps")
	ncclPerftestCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")

	return ncclPerftestCmd
}
