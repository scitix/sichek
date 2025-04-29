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

func RunLocalIBBW(ibBwPerfType string, ibDevice string, msgSize int, testDuring int, verbose bool) (float64, error) {
	ibBwArgs := fmt.Sprintf("-s %d -D %d -x 0 -F --report_gbits -d %s -q 2", msgSize, testDuring, ibDevice)

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

	return 0, fmt.Errorf("failed to parse bandwidth for %s", ibDevice)
}

func RunLocalNumaAwareIBBW(ibBwPerfType string, ibDevice string, numaNode int, numaCpu0 int, msgSize int, testDuring int, verbose bool) (float64, error) {
	ibBwArgs := fmt.Sprintf("-s %d -D %d -x 0 -F --report_gbits -d %s -q 2", msgSize, testDuring, ibDevice)
	// Start server process
	runCmd := fmt.Sprintf("numactl --membind=%d --cpunodebind=%d %s %s > /dev/null", numaNode, numaNode, ibBwPerfType, ibBwArgs)
	if verbose {
		fmt.Printf("Executing: %s\n", runCmd)
	}
	serverCmd := exec.Command("sh", "-c", runCmd)
	if err := serverCmd.Start(); err != nil {
		if verbose {
			fmt.Printf("error starting server: %v", err)
		}
		return 0, fmt.Errorf("error starting server: %v", err)
	}
	// Sleep to allow server to start
	if verbose {
		fmt.Println("Sleeping for 2 seconds to allow server to initialize")
	}
	time.Sleep(2 * time.Second)
	// Start client process
	// numaCpu1 := numaCpu0 + 1
	runCmd = fmt.Sprintf("numactl --membind=%d --cpunodebind=%d %s %s 127.0.0.1", numaNode, numaNode, ibBwPerfType, ibBwArgs)
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
	return 0, fmt.Errorf("failed to parse bandwiidth for %s", ibDevice)

}

func getActiveIBDevices() ([]collector.IBHardWareInfo, error) {
	var info collector.InfinibandInfo
	ibDevs := info.GetIBdevs()
	var activeDevices []collector.IBHardWareInfo
	for _, IBDev := range ibDevs {
		var hwInfo collector.IBHardWareInfo
		if len(info.GetPhyStat(IBDev)) >= 1 {
			hwInfo.PhyState = info.GetPhyStat(IBDev)[0]
			hwInfo.PortState = info.GetIBStat(IBDev)[0]
			hwInfo.NumaNode = info.GetNumaNode(IBDev)[0]
			hwInfo.CPULists = info.GetCPUList(IBDev)[0]
			hwInfo.IBDev = IBDev
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

func CheckNodeIBPerfHealth(ibBwPerfType string, expectedBandwidthGbps float64, ibDevice string, msgSize int, testDuring int, numaAware bool, verbose bool) (*common.Result, error) {
	activeDeviceInfos := make([]collector.IBHardWareInfo, 0)
	var err error
	if numaAware || ibDevice == "" {
		activeDeviceInfos, err = getActiveIBDevices()
		if err != nil {
			return nil, fmt.Errorf("getActiveIBDevices error :%v", err)
		}
	}
	usedDeviceInfos := make([]collector.IBHardWareInfo, 0)
	for dev := range activeDeviceInfos {
		if ibDevice == "" || ibDevice == activeDeviceInfos[dev].IBDev {
			usedDeviceInfos = append(usedDeviceInfos, activeDeviceInfos[dev])
		}
	}
	status := consts.StatusNormal
	checkRes := make([]*common.CheckerResult, 0)
	for _, dev := range usedDeviceInfos {
		resItem := IbPerfCheckItems[IbPerfCheckerName]
		var bw float64
		var err error
		if numaAware {
			numaNode, err := strconv.Atoi(dev.NumaNode)
			if err != nil {
				return nil, fmt.Errorf("error converting numa node string %s to int: %v", dev.NodeGUID, err)
			}
			// Split the first element on "-" and take the first part
			parts := strings.Split(dev.CPULists, ",")
			numaCPUStr := strings.Split(parts[0], "-")
			numaCPU0, err := strconv.Atoi(numaCPUStr[0])
			if err != nil {
				return nil, fmt.Errorf("error converting numa cpu string %s toint: %v", numaCPUStr[0], err)
			}
			fmt.Printf("NUMA %d CPU 0 for device %v is: %d\n", numaNode, dev, numaCPU0)
			bw, err = RunLocalNumaAwareIBBW(ibBwPerfType, dev.IBDev, numaNode, numaCPU0, msgSize, testDuring, verbose)
		} else {
			bw, err = RunLocalIBBW(ibBwPerfType, dev.IBDev, msgSize, testDuring, verbose)
		}
		if err != nil {
			resItem.Detail = fmt.Sprintf("❌ Test failed on %s ,err : %v", dev, err)
			resItem.Status = consts.StatusAbnormal
			status = resItem.Status
			checkRes = append(checkRes, resItem)
			continue
		}
		if verbose {
			fmt.Printf("IB Device %s - Bandwidth: %.2f Gbps\n", dev, bw)
		}
		if bw < expectedBandwidthGbps {
			resItem.Detail = fmt.Sprintf("❌ IB Device %s (%.2f Gbps) does NOT meet the required %.2f Gbps bandwidth\n", dev, bw, expectedBandwidthGbps)
			resItem.Status = consts.StatusAbnormal
			status = resItem.Status
		} else {
			resItem.Detail = fmt.Sprintf("✅ IB Device %s PASSED with %.2f Gbps\n", dev, bw)
		}
		checkRes = append(checkRes, resItem)
	}
	res := &common.Result{
		Item:     "Ib Perf Test",
		Status:   status,
		Checkers: checkRes,
	}
	return res, nil

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
