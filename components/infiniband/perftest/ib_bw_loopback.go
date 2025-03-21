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

	"github.com/scitix/sichek/components/infiniband/collector"
)

// 设定最低通过带宽要求 (单位: Gbps)
const ExpectedBandwidthGbps = 50.0

func runLocalIBBW(ibDevice string, msgSize int, ibBwPerfType string, testDuring int) (float64, error) {
	ibBwArgs := fmt.Sprintf("-s %d -D %d -x 0 -F --report_gbits -d %s -q 2", msgSize, testDuring, ibDevice)

	// Start server process
	runCmd := fmt.Sprintf("%s %s > /dev/null", ibBwPerfType, ibBwArgs)
	fmt.Printf("Executing: %s\n", runCmd)
	serverCmd := exec.Command("sh", "-c", runCmd)
	if err := serverCmd.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return 0, fmt.Errorf("Error starting server: %v\n", err)
	}

	// Sleep to allow server to start
	fmt.Println("Sleeping for 2 seconds to allow server to initialize")
	time.Sleep(2 * time.Second)

	// Start client process
	runCmd = fmt.Sprintf("%s %s 127.0.0.1", ibBwPerfType, ibBwArgs)
	fmt.Printf("Executing: %s\n", runCmd)
	clientCmd := exec.Command("sh", "-c", runCmd)

	var stdout, stderr bytes.Buffer
	clientCmd.Stdout = &stdout
	clientCmd.Stderr = &stderr

	if err := clientCmd.Run(); err != nil {
		fmt.Printf("Error executing client: %v\n", err)
		fmt.Printf("Stdout: %s\nStderr: %s\n", stdout.String(), stderr.String())
		return 0, fmt.Errorf("Error executing client: %v\n", err)
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

func getActiveIBDevices() ([]string, error) {
	var info collector.InfinibandInfo
	ibDevs := info.GetIBdevs()
	activeDevices := []string{}
	for _, IBDev := range ibDevs {
		var hwInfo collector.IBHardWareInfo
		if len(info.GetPhyStat(IBDev)) >= 1 {
			hwInfo.PhyState = info.GetPhyStat(IBDev)[0]
		}
		if len(info.GetIBStat(IBDev)) >= 1 {
			hwInfo.PortState = info.GetIBStat(IBDev)[0]
		}
		fmt.Printf("%s: %s %s\n", IBDev, hwInfo.PhyState, hwInfo.PortState)
		if strings.Contains(hwInfo.PhyState, "LinkUp") && strings.Contains(hwInfo.PortState, "ACTIVE") {
			activeDevices = append(activeDevices, IBDev)
		}
	}

	if len(activeDevices) == 0 {
		return nil, fmt.Errorf("no active IB devices found")
	}

	return activeDevices, nil
}

func CheckNodeIBPerfHealth() error {
	ibDevices, err := getActiveIBDevices()
	if err != nil {
		fmt.Printf("Failed to get IB devices: %v\n", err)
		return nil
	}

	fmt.Printf("Found IB devices: %v\n", ibDevices)

	results := make(map[string]float64)
	nodePass := true

	for _, dev := range ibDevices {
		bw, err := runLocalIBBW(dev, 1048576, "ib_read_bw", 1)
		if err != nil {
			fmt.Printf("❌ Test failed on %s: %v", dev, err)
			nodePass = false
			continue
		}

		results[dev] = bw
		fmt.Printf("IB Device %s - Bandwidth: %.2f Gbps\n", dev, bw)

		if bw < ExpectedBandwidthGbps {
			fmt.Printf("❌ IB Device %s does NOT meet the required %.2f Gbps bandwidth\n", dev, ExpectedBandwidthGbps)
			nodePass = false
		} else {
			fmt.Printf("✅ IB Device %s PASSED with %.2f Gbps\n", dev, bw)
		}
	}

	if nodePass {
		fmt.Println("✅ Node IB Health Check PASSED: All IB devices meet the bandwidth requirement.")
		return nil
	} else {
		fmt.Println("❌ Node IB Health Check FAILED: One or more IB devices did not meet the required bandwidth.")
		return fmt.Errorf("Node IB Health Check FAILED: One or more IB devices did not meet the required bandwidth")
	}
}