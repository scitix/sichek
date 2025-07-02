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
package perftest

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
)

func RunLocalIBBW(ibBwPerfType string, ibDevice1 string, ibDevice2 string, msgSize int, testDuring, gid int, useGDR, verbose bool) (float64, error) {
	ibBwArgs := fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s -q 2", msgSize, testDuring, gid, ibDevice1)
	if useGDR {
		ibBwArgs += " --use_cuda"
	}
	// Start server process
	runCmd := fmt.Sprintf("%s %s > /dev/null", ibBwPerfType, ibBwArgs)
	if verbose {
		fmt.Printf("Executing: %s\n", runCmd)
	}
	serverCmd := exec.Command("sh", "-c", runCmd)
	if err := serverCmd.Start(); err != nil {
		if verbose {
			fmt.Printf("Error starting server: %v\n", err)
		}
		return 0, fmt.Errorf("error starting server: %v", err)
	}

	// Sleep to allow server to start
	if verbose {
		fmt.Println("Sleeping for 2 seconds to allow server to initialize")
	}
	time.Sleep(2 * time.Second)

	// Start client process
	ibBwArgs = fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s -q 2", msgSize, testDuring, gid, ibDevice2)
	if useGDR {
		ibBwArgs += " --use_cuda"
	}
	runCmd = fmt.Sprintf("%s %s 127.0.0.1", ibBwPerfType, ibBwArgs)
	if verbose {
		fmt.Printf("Executing: %s\n", runCmd)
	}
	clientCmd := exec.Command("sh", "-c", runCmd)

	var stdout, stderr bytes.Buffer
	clientCmd.Stdout = &stdout
	clientCmd.Stderr = &stderr

	if err := clientCmd.Run(); err != nil {
		if verbose {
			fmt.Printf("Error executing client: %v\n", err)
			fmt.Printf("Stdout: %s\nStderr: %s\n", stdout.String(), stderr.String())
		}
		return 0, fmt.Errorf("error executing client: %v", err)
	}

	// Parse bandwidth from output
	lines := strings.Split(stdout.String(), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[0] == strconv.Itoa(msgSize) {
			bw, err := strconv.ParseFloat(fields[3], 64)
			if err == nil {
				return bw, nil
			}
		}
	}

	return 0, fmt.Errorf("failed to parse bandwidth for %s", ibDevice1)
}

func RunLocalNumaAwareIBBW(
	ibBwPerfType string,
	srcDev, dstDev collector.IBHardWareInfo,
	msgSize int,
	testDuring, gid int,
	useGDR, verbose bool,
) (float64, error) {
	numaNode, err := strconv.Atoi(srcDev.NumaNode)
	if err != nil {
		return 0, fmt.Errorf("invalid NUMA node %q for device %s: %v", srcDev.NumaNode, srcDev.IBDev, err)
	}
	// parts := strings.Split(srcDev.CPULists, ",")
	// numaCPU0, err := strconv.Atoi(strings.Split(parts[0], "-")[0])
	if err != nil {
		return 0, fmt.Errorf("invalid CPUList %q for device %s: %v", srcDev.CPULists, srcDev.IBDev, err)
	}

	// server
	serverArgs := fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s -q 2", msgSize, testDuring, gid, srcDev.IBDev)
	if useGDR {
		serverArgs += " --use_cuda"
	}
	serverCmdStr := fmt.Sprintf("numactl --membind=%d --cpunodebind=%d %s %s > /dev/null", numaNode, numaNode, ibBwPerfType, serverArgs)
	if verbose {
		fmt.Printf("Executing server: %s\n", serverCmdStr)
	}

	serverCmd := exec.Command("sh", "-c", serverCmdStr)
	if err := serverCmd.Start(); err != nil {
		return 0, fmt.Errorf("error starting server: %v", err)
	}

	if verbose {
		fmt.Println("Sleeping for 2 seconds to allow server to initialize")
	}
	time.Sleep(2 * time.Second)

	// client
	clientArgs := fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s -q 2", msgSize, testDuring, gid, dstDev.IBDev)
	if useGDR {
		clientArgs += " --use_cuda"
	}
	clientCmdStr := fmt.Sprintf("numactl --membind=%d --cpunodebind=%d %s %s 127.0.0.1", numaNode, numaNode, ibBwPerfType, clientArgs)
	if verbose {
		fmt.Printf("Executing client: %s\n", clientCmdStr)
	}

	clientCmd := exec.Command("sh", "-c", clientCmdStr)
	var stdout, stderr bytes.Buffer
	clientCmd.Stdout = &stdout
	clientCmd.Stderr = &stderr

	if err := clientCmd.Run(); err != nil {
		if verbose {
			fmt.Printf("Error executing client: %v\n", err)
			fmt.Printf("Stdout: %s\nStderr: %s\n", stdout.String(), stderr.String())
		}
		return 0, fmt.Errorf("error executing client: %v", err)
	}

	// Parse bandwidth from output
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[0] == strconv.Itoa(msgSize) {
			bw, err := strconv.ParseFloat(fields[3], 64)
			if err == nil {
				return bw, nil
			}
		}
	}
	return 0, fmt.Errorf("failed to parse bandwidth for %s -> %s", srcDev.IBDev, dstDev.IBDev)
}

func getActiveIBDevices() ([]collector.IBHardWareInfo, error) {
	var info collector.InfinibandInfo
	ibDevs := info.GetIBdevs()
	var activeDevices []collector.IBHardWareInfo
	for mlxDev := range ibDevs {
		var hwInfo collector.IBHardWareInfo
		if len(info.GetPhyStat(mlxDev)) >= 1 {
			hwInfo.PhyState = info.GetPhyStat(mlxDev)[0]
			hwInfo.PortState = info.GetIBStat(mlxDev)[0]
			hwInfo.NumaNode = info.GetNumaNode(mlxDev)[0]
			hwInfo.CPULists = info.GetCPUList(mlxDev)[0]
			hwInfo.IBDev = mlxDev
		}

		if strings.Contains(hwInfo.PhyState, "LinkUp") && strings.Contains(hwInfo.PortState, "ACTIVE") {
			activeDevices = append(activeDevices, hwInfo)
			fmt.Println("Active IB device found:", hwInfo)
		}
	}

	if len(activeDevices) == 0 {
		return nil, fmt.Errorf("no active IB devices found")
	}

	return activeDevices, nil
}

func CheckNodeIBPerfHealth(
	ibBwPerfType string,
	expectedBandwidthGbps float64,
	ibDevice string,
	msgSize int,
	testDuring int,
	gid int,
	numaAware bool,
	useGDR, verbose bool,
) (*common.Result, error) {
	activeDeviceInfos := make([]collector.IBHardWareInfo, 0)
	var err error
	activeDeviceInfos, err = getActiveIBDevices()
	if err != nil {
		return nil, fmt.Errorf("getActiveIBDevices error :%v", err)
	}

	deviceFilter := make(map[string]struct{})
	if ibDevice != "" {
		devices := strings.Split(ibDevice, ",")
		for _, d := range devices {
			deviceFilter[strings.TrimSpace(d)] = struct{}{}
		}
	}
	usedDeviceInfos := make([]collector.IBHardWareInfo, 0)
	for _, dev := range activeDeviceInfos {
		if ibDevice == "" {
			usedDeviceInfos = append(usedDeviceInfos, dev)
		} else {
			if _, ok := deviceFilter[dev.IBDev]; ok {
				usedDeviceInfos = append(usedDeviceInfos, dev)
			}
		}
	}

	status := consts.StatusNormal
	checkRes := make([]*common.CheckerResult, 0)

	for _, srcDev := range usedDeviceInfos {
		for _, dstDev := range usedDeviceInfos {
			resTemplate := PerfCheckItems[IbPerfCheckerName]
			resItem := &common.CheckerResult{
				Name:   resTemplate.Name,
				Status: consts.StatusNormal,
			}

			resItem.Detail = fmt.Sprintf("Testing %s -> %s", srcDev.IBDev, dstDev.IBDev)

			var bw float64
			var err error

			if numaAware {
				bw, err = RunLocalNumaAwareIBBW(ibBwPerfType, srcDev, dstDev, msgSize, testDuring, gid, useGDR, verbose)
			} else {
				bw, err = RunLocalIBBW(ibBwPerfType, srcDev.IBDev, dstDev.IBDev, msgSize, testDuring, gid, useGDR, verbose)
			}

			if err != nil {
				resItem.Status = consts.StatusAbnormal
				resItem.Detail += fmt.Sprintf(" ❌ Test failed: %v", err)
				status = resItem.Status
			} else if bw < expectedBandwidthGbps {
				resItem.Status = consts.StatusAbnormal
				resItem.Detail += fmt.Sprintf(" ❌ %.2f Gbps < expected %.2f Gbps", bw, expectedBandwidthGbps)
				status = resItem.Status
			} else {
				resItem.Status = consts.StatusNormal
				resItem.Detail += fmt.Sprintf(" ✅ %.2f Gbps", bw)
			}

			if verbose {
				fmt.Println(resItem.Detail)
			}
			checkRes = append(checkRes, resItem)
		}
	}

	return &common.Result{
		Item:     "Ib Perf Test",
		Status:   status,
		Checkers: checkRes,
	}, nil
}

func PrintInfo(result *common.Result, verbos bool) bool {
	checkerResults := result.Checkers
	if result.Status == consts.StatusNormal {
		fmt.Println("✅ Node IB Health Check PASSED: All IB devices meet the bandwidth requirement.")
		return true
	}
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			fmt.Printf("%s\n", result.Detail)
		} else {
			if verbos {
				fmt.Printf("%s\n", result.Detail)
			}
		}
	}
	return false
}
