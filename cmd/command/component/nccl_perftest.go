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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Config struct {
	NumGpus     int
	Gpulist     string
	TestBin     string
	beginBuffer string
	endBuffer   string
	DisableNvls bool
}

func NewNcclPerftestCmd() *cobra.Command {

	ncclPerftestCmd := &cobra.Command{
		Use:   "nccltest",
		Short: "Perform Nccl performance tests",
		Run: func(cmd *cobra.Command, args []string) {
			_, cancel := context.WithTimeout(context.Background(), 120*time.Second)

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
			if expectedBandwidthGbps == 0 {
				nvidiaSpecCfg, err := config.LoadSpec("/var/sichek/config/default_spec.yaml")
				if err != nil {
					logrus.WithField("perftest", "nccl").Errorf("failed to load default spec: %v", err)
				} else {
					if nvidiaSpecCfg.Perf.NcclAllReduceBw > 0 {
						expectedBandwidthGbps = nvidiaSpecCfg.Perf.NcclAllReduceBw
						fmt.Printf("Using default expected bandwidth: %.2f Gbps\n", expectedBandwidthGbps)
					} else {
						fmt.Println("No expected bandwidth specified, using 0 Gbps")
					}
				}
			}
			timeout, err := cmd.Flags().GetInt("timeout")
			if err != nil {
				logrus.WithField("perftest", "nccl").Error(err)
				return
			}
			var res *common.Result
			result := 0
			fmt.Printf("Running NCCL performance test with %d GPUs, begin buffer: %s, end buffer: %s, disable NVLinks: %t, expected bandwidth: %.2f Gbps\n", numGpus, beginBuffer, endBuffer, disableNvls, expectedBandwidthGbps)
			if scale {
				for g := 2; g <= numGpus; g++ {
					res, err = CheckNcclPerf(g, gpulist, beginBuffer, endBuffer, disableNvls, expectedBandwidthGbps, timeout)
					if err != nil {
						logrus.WithField("perftest", "nccl").Error(err)
						result = -1
					}
				}
			} else {
				res, err = CheckNcclPerf(numGpus, gpulist, beginBuffer, endBuffer, disableNvls, expectedBandwidthGbps, timeout)
				if err != nil {
					logrus.WithField("perftest", "nccl").Error(err)
					result = -1
				}
			}
			if result == 0 {
				passed := PrintNcclPerfInfo(res)
				ComponentStatuses[res.Item] = passed
			} else {
				ComponentStatuses["NcclPerf"] = false
			}
		},
	}

	ncclPerftestCmd.Flags().IntP("num-gpus", "n", 8, "Max GPU numbers to test")
	ncclPerftestCmd.Flags().StringP("gpulist", "g", "", "specific GPU list to test, e.g. 0,1,2,3")
	ncclPerftestCmd.Flags().StringP("begin", "b", "8", "begin buffer")
	ncclPerftestCmd.Flags().StringP("end", "e", "8", "end buffer")
	ncclPerftestCmd.Flags().Bool("scale-gpus", false, "Run NCCL test scaling GPU count from 2 to n")
	ncclPerftestCmd.Flags().BoolP("disable-nvls", "d", false, "test without nvlinks")
	ncclPerftestCmd.Flags().Float64("expect-bw", 0, "Expected bandwidth in Gbps")
	ncclPerftestCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	ncclPerftestCmd.Flags().IntP("timeout", "t", 120, "Timeout in seconds")

	return ncclPerftestCmd
}

func GetDefaultNcclTestPath(testBin string) (string, error) {
	defaultScriptsDirPath := filepath.Join(consts.DefaultProductionPath, "scripts", testBin)
	_, err := os.Stat(defaultScriptsDirPath)
	if err == nil {
		return defaultScriptsDirPath, nil
	}
	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("get curr file path failed")
	}
	upperDir := filepath.Dir(filepath.Dir(curFile)) // 两级上层目录
	defaultScriptsDirPath = filepath.Join(upperDir, "scripts", testBin)
	return defaultScriptsDirPath, nil
}

func buildNcclTestCmd(cfg Config) *exec.Cmd {
	// nccl-tests
	testPath, err := GetDefaultNcclTestPath(cfg.TestBin)
	if err != nil {
		logrus.WithField("perftest", "nccl").Errorf("GetDefaultNcclTestPath error :%v\n", err)
		return nil
	}
	args := []string{
		testPath,
	}
	if cfg.NumGpus != 0 {
		args = append(args, fmt.Sprintf("-g %d", cfg.NumGpus))
	}
	if cfg.beginBuffer != "" {
		args = append(args, fmt.Sprintf("-b %s", cfg.beginBuffer))
	}
	if cfg.endBuffer != "" {
		args = append(args, fmt.Sprintf("-e %s", cfg.endBuffer))
	}
	fmt.Printf("== Run %d GPU nccl all_reduce test ==\n", cfg.NumGpus)
	cmd := exec.Command("bash", args...)

	// Start with current environment variables to inherit all existing env vars
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Override or add specific environment variables
	if cfg.DisableNvls {
		envMap["NCCL_NVLS_ENABLE"] = "0"
	} else {
		envMap["NCCL_NVLS_ENABLE"] = "1"
	}
	if cfg.Gpulist != "" {
		fmt.Printf("CUDA_VISIBLE_DEVICES: %s\n", cfg.Gpulist)
		envMap["CUDA_VISIBLE_DEVICES"] = cfg.Gpulist
	}
	envMap["UCX_TLS"] = ""
	envMap["OMPI_MCA_pml"] = "^ucx"

	// Convert map back to slice of "KEY=VALUE" format
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	logrus.WithField("perftest", "nccl").Infof("env: %v\n", env)
	cmd.Env = env
	return cmd
}

func runNcclTest(cfg Config, timeout int) ([]float64, error) {
	cmd := buildNcclTestCmd(cfg)
	if cmd == nil {
		return nil, fmt.Errorf("failed to build nccl test command")
	}
	logrus.WithField("perftest", "nccl").Infof("Command: %s\n", cmd.String())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start nccl test: %w", err)
	}

	var (
		stdoutBuf  bytes.Buffer
		stderrBuf  bytes.Buffer
		stdoutDone = make(chan struct{})
		stderrDone = make(chan struct{})
	)

	// async copy the output streams to the buffers and print the real-time output
	go func() {
		defer close(stdoutDone)
		mw := io.MultiWriter(os.Stdout, &stdoutBuf)
		_, _ = io.Copy(mw, stdout)
	}()
	go func() {
		defer close(stderrDone)
		mw := io.MultiWriter(os.Stderr, &stderrBuf)
		_, _ = io.Copy(mw, stderr)
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		// wait for stdout and stderr to be done
		<-stdoutDone
		<-stderrDone
		if err != nil {
			stderrStr := stderrBuf.String()
			// logrus.WithField("perftest", "nccl").Errorf("nccl test failed: %v, stdout: %s, stderr: %s", err, outputStr, stderrStr)
			return nil, fmt.Errorf("nccl test command failed: %v. stderr: %s", err, stderrStr)
		}
	case <-time.After(time.Duration(timeout) * time.Second):
		// kill the process if it timed out
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		// wait for the command to complete and the output streams to be copied
		<-done // wait for Wait() to complete
		<-stdoutDone
		<-stderrDone
		stderrStr := stderrBuf.String()
		return nil, fmt.Errorf("nccl test timed out after %d seconds. stderr: %s", timeout, stderrStr)
	}

	outputStr := stdoutBuf.String()
	logrus.WithField("perftest", "nccl").Infof("output: %s\n", outputStr)

	var res []float64
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Avg bus bandwidth") {
			parts := strings.Split(line, ":")
			if len(parts) < 2 {
				continue
			}
			bwStr := strings.TrimSpace(parts[1])
			bwStr = strings.Split(bwStr, " ")[0]
			bw, err := strconv.ParseFloat(bwStr, 64)
			if err != nil {
				return nil, fmt.Errorf("parse bandwidth err: %v. Output: %s", err, outputStr)
			}
			res = append(res, bw)
		}
	}
	return res, nil
}

func checkBandwidth(avgBusBandwidths []float64, exceptBwGbps float64) *common.Result {
	var sum float64

	resItem := &common.CheckerResult{
		Name:        "NCCLPerfTest",
		Description: "",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "NcclPerfTestError",
		Suggestion:  "Check Nccl Bandwidth",
	}

	for _, bw := range avgBusBandwidths {
		sum += bw
	}
	avgBusBandwidth := sum / float64(len(avgBusBandwidths))

	if avgBusBandwidth < exceptBwGbps {
		resItem.Status = consts.StatusAbnormal
		resItem.Detail = fmt.Sprintf("NCCL allreduce bandwidth test failed, avgBusBandwidth returned %.2f Gbps, but expected > %.2f Gbps.\n", avgBusBandwidth, exceptBwGbps)

	} else {
		resItem.Status = consts.StatusNormal
		resItem.Detail = fmt.Sprintf("NCCL allreduce bandwidth test passed, avgBusBandwidth = %.2f Gbps.\n", avgBusBandwidth)
	}
	res := &common.Result{
		Item:     "NcclPerf",
		Status:   resItem.Status,
		Level:    resItem.Level,
		Checkers: []*common.CheckerResult{resItem},
	}
	return res

}

func CheckNcclPerf(numGpus int, gpulist, beginBuffer, endBuffer string, disableNvls bool, exceptBwGbps float64, timeout int) (*common.Result, error) {
	jobCfg := Config{
		NumGpus:     numGpus,
		Gpulist:     gpulist,
		TestBin:     "nccl_perf",
		DisableNvls: disableNvls,
		beginBuffer: beginBuffer,
		endBuffer:   endBuffer,
	}
	records, err := runNcclTest(jobCfg, timeout)
	if err != nil {
		return nil, fmt.Errorf("run nccl test fail: %v", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("get no avg bus bandwidth res")
	}
	res := checkBandwidth(records, exceptBwGbps)

	return res, nil
}

func PrintNcclPerfInfo(result *common.Result) bool {
	checkerResults := result.Checkers
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			fmt.Printf("%s%s%s\n", consts.Red, result.Detail, consts.Reset)
			return false
		} else {
			fmt.Printf("%s%s%s\n", consts.Green, result.Detail, consts.Reset)
			return true
		}
	}
	return true
}
