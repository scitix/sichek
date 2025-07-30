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

func RunLocalIBTest(ibBwPerfType string, ibDevice1 string, ibDevice2 string, msgSize int, testDuring, gid, qpNum int, useGDR, verbose bool) (string, error) {
	ibBwArgs := fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s", msgSize, testDuring, gid, ibDevice1)
	if strings.Contains(ibBwPerfType, "bw") {
		ibBwArgs += fmt.Sprintf(" -q %d", qpNum)
	}
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
		return "", fmt.Errorf("error starting server: %v", err)
	}

	// Sleep to allow server to start
	if verbose {
		fmt.Println("Sleeping for 2 seconds to allow server to initialize")
	}
	time.Sleep(2 * time.Second)

	// Start client process
	ibBwArgs = fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s", msgSize, testDuring, gid, ibDevice2)
	if strings.Contains(ibBwPerfType, "bw") {
		ibBwArgs += fmt.Sprintf(" -q %d", qpNum)
	}
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
		return "", fmt.Errorf("error executing client: %v", err)
	}
	return stdout.String(), nil
}

func RunLocalNumaAwareIBTest(
	ibBwPerfType string,
	srcDev, dstDev collector.IBHardWareInfo,
	msgSize int,
	testDuring, gid, qpNum int,
	useGDR, verbose bool,
) (string, error) {
	numaNode, err := strconv.Atoi(srcDev.NumaNode)
	if err != nil {
		return "", fmt.Errorf("invalid NUMA node %q for device %s: %v", srcDev.NumaNode, srcDev.IBDev, err)
	}

	// server
	serverArgs := fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s", msgSize, testDuring, gid, srcDev.IBDev)
	if strings.Contains(ibBwPerfType, "bw") {
		serverArgs += fmt.Sprintf(" -q %d", qpNum)
	}
	if useGDR {
		serverArgs += " --use_cuda"
	}
	serverCmdStr := fmt.Sprintf("numactl --membind=%d --cpunodebind=%d %s %s > /dev/null", numaNode, numaNode, ibBwPerfType, serverArgs)
	if verbose {
		fmt.Printf("Executing server: %s\n", serverCmdStr)
	}

	serverCmd := exec.Command("sh", "-c", serverCmdStr)
	if err := serverCmd.Start(); err != nil {
		return "", fmt.Errorf("error starting server: %v", err)
	}

	if verbose {
		fmt.Println("Sleeping for 2 seconds to allow server to initialize")
	}
	time.Sleep(2 * time.Second)

	// client
	clientArgs := fmt.Sprintf("-s %d -D %d -x %d -F --report_gbits -d %s", msgSize, testDuring, gid, dstDev.IBDev)
	if strings.Contains(ibBwPerfType, "bw") {
		serverArgs += fmt.Sprintf(" -q %d", qpNum)
	}
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
		return "", fmt.Errorf("error executing client: %v", err)
	}
	return stdout.String(), nil
}

func parseBandwidth(out string, msgSize int, srcDev, dstDev string) (float64, error) {
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[0] == strconv.Itoa(msgSize) {
			return strconv.ParseFloat(fields[3], 64)
		}
	}
	return 0, fmt.Errorf("failed to parse bandwidth for %s -> %s msgSize=%d", srcDev, dstDev, msgSize)
}

func parseLatency(out string, msgSize int, srcDev, dstDev string) (float64, error) {
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == strconv.Itoa(msgSize) {
			return strconv.ParseFloat(fields[2], 64)
		}
	}
	return 0, fmt.Errorf("failed to parse latency for %s -> %s msgSize=%d", srcDev, dstDev, msgSize)
}

func getActiveIBPFDevices() ([]collector.IBHardWareInfo, []collector.IBHardWareInfo, error) {
	var info collector.InfinibandInfo
	ibDevs := info.GetIBPFdevs()
	var activeDevices []collector.IBHardWareInfo
	var deactiveDevices []collector.IBHardWareInfo
	for mlxDev := range ibDevs {
		var hwInfo collector.IBHardWareInfo
		hwInfo.IBDev = mlxDev
		phyStats := info.GetPhyStat(mlxDev)
		if len(phyStats) > 0 {
			hwInfo.PhyState = phyStats[0]
		} else {
			hwInfo.PhyState = "UNKNOWN"
		}

		portStates := info.GetIBStat(mlxDev)
		if len(portStates) > 0 {
			hwInfo.PortState = portStates[0]
		} else {
			hwInfo.PortState = "UNKNOWN"
		}

		numaNodes := info.GetNumaNode(mlxDev)
		if len(numaNodes) > 0 {
			hwInfo.NumaNode = numaNodes[0]
		}

		cpuLists := info.GetCPUList(mlxDev)
		if len(cpuLists) > 0 {
			hwInfo.CPULists = cpuLists[0]
		}

		if strings.Contains(hwInfo.PhyState, "LinkUp") && strings.Contains(hwInfo.PortState, "ACTIVE") {
			activeDevices = append(activeDevices, hwInfo)
			fmt.Println("Active IB device found:", hwInfo)
		} else {
			deactiveDevices = append(deactiveDevices, hwInfo)
			fmt.Println("Deactive IB device found:", hwInfo)
		}
	}

	if len(activeDevices) == 0 {
		return nil, deactiveDevices, fmt.Errorf("no active IB devices found")
	}

	return activeDevices, deactiveDevices, nil
}

func CheckNodeIBPerfHealth(
	ibBwPerfType string,
	expectedBandwidthGbps, expectedLatencyUs float64,
	ibDevice string,
	msgSize int,
	testDuring int,
	gid, qpNum int,
	numaAware bool,
	useGDR, verbose bool,
) (*common.Result, error) {
	activeDeviceInfos, deactiveDeviceInfos, err := getActiveIBPFDevices()
	if err != nil {
		return nil, fmt.Errorf("getActiveIBPFDevices error :%v", err)
	}

	deviceFilter := make(map[string]struct{})
	if ibDevice != "" {
		devices := strings.Split(ibDevice, ",")
		for _, d := range devices {
			deviceFilter[strings.TrimSpace(d)] = struct{}{}
		}
	}
	usedDeviceInfos := make([]collector.IBHardWareInfo, 0)
	var usedDeviceStrings []string
	for _, dev := range activeDeviceInfos {
		if ibDevice == "" {
			usedDeviceInfos = append(usedDeviceInfos, dev)
			usedDeviceStrings = append(usedDeviceStrings, dev.IBDev)
		} else {
			if _, ok := deviceFilter[dev.IBDev]; ok {
				usedDeviceInfos = append(usedDeviceInfos, dev)
				usedDeviceStrings = append(usedDeviceStrings, dev.IBDev)
			}
		}
	}

	status := consts.StatusNormal
	checkRes := make([]*common.CheckerResult, 0)

	fmt.Printf("Using IB devices for performance test: %v\n", usedDeviceStrings)

	for _, srcDev := range usedDeviceInfos {
		for _, dstDev := range usedDeviceInfos {
			resTemplate := PerfCheckItems[IBPerfTestName]
			resItem := &common.CheckerResult{
				Name:   resTemplate.Name,
				Status: consts.StatusNormal,
			}

			resItem.Detail = fmt.Sprintf("Testing %s -> %s", srcDev.IBDev, dstDev.IBDev)

			var out string
			var metrics float64
			var err error
			if numaAware {
				out, err = RunLocalNumaAwareIBTest(ibBwPerfType, srcDev, dstDev, msgSize, testDuring, gid, qpNum, useGDR, verbose)
			} else {
				out, err = RunLocalIBTest(ibBwPerfType, srcDev.IBDev, dstDev.IBDev, msgSize, testDuring, gid, qpNum, useGDR, verbose)
			}
			if err == nil {
				if strings.Contains(ibBwPerfType, "lat") {
					metrics, err = parseLatency(out, msgSize, srcDev.IBDev, dstDev.IBDev)
					if err != nil || metrics > expectedLatencyUs {
						resItem.Status = consts.StatusAbnormal
						resItem.Detail += fmt.Sprintf(" ❌ %.2f us > expected %.2f us", metrics, expectedLatencyUs)
						status = consts.StatusAbnormal
					} else {
						resItem.Status = consts.StatusNormal
						resItem.Detail += fmt.Sprintf(" ✅ %.2f <= expected %.2f us", metrics, expectedLatencyUs)
					}
				} else {
					metrics, err = parseBandwidth(out, msgSize, srcDev.IBDev, dstDev.IBDev)
					if err != nil || metrics < expectedBandwidthGbps {
						resItem.Status = consts.StatusAbnormal
						resItem.Detail += fmt.Sprintf(" ❌ %.2f Gbps < expected %.2f Gbps", metrics, expectedBandwidthGbps)
						status = consts.StatusAbnormal
					} else {
						resItem.Status = consts.StatusNormal
						resItem.Detail += fmt.Sprintf(" ✅ %.2f Gbps >= expected %.2f Gbps", metrics, expectedBandwidthGbps)
					}
				}
			} else {
				resItem.Status = consts.StatusAbnormal
				resItem.Detail += fmt.Sprintf(" ❌ Test failed: %v", err)
				status = consts.StatusAbnormal
			}

			fmt.Println(resItem.Detail)
			checkRes = append(checkRes, resItem)
		}
	}

	if len(deactiveDeviceInfos) != 0 {
		resItem := &common.CheckerResult{
			Name:        IBPerfTestName,
			Description: "Deactive IB devices found when perform bandwidth test",
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      fmt.Sprintf("deactive IB devices found: %v", usedDeviceInfos),
			ErrorName:   "DeactiveIBDevicesFound",
			Suggestion:  "Check IB device connections and configurations",
		}
		checkRes = append(checkRes, resItem)
		status = consts.StatusAbnormal
	}

	if len(deviceFilter) != 0 && len(usedDeviceInfos) != len(deviceFilter) {
		resItem := &common.CheckerResult{
			Name:        IBPerfTestName,
			Description: "Not all IB devices found are active to perform bandwidth test",
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      fmt.Sprintf("Not all IB devices found: %v, while %s are specificd ", usedDeviceStrings, ibDevice),
			ErrorName:   "DeactiveIBDevicesFound",
			Suggestion:  "Check IB device connections and configurations",
		}
		checkRes = append(checkRes, resItem)
		status = consts.StatusAbnormal
	}
	return &common.Result{
		Item:     IBPerfTestName,
		Status:   status,
		Checkers: checkRes,
	}, nil
}

func PrintInfo(result *common.Result, verbos bool) bool {
	if result == nil {
		fmt.Println("No IB performance test results found.")
		return false
	}
	checkerResults := result.Checkers
	if result.Status == consts.StatusNormal {
		fmt.Println("✅ Node IB Health Check PASSED: All IB devices meet the spec.")
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
