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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type RoCEV2DeviceInfo struct {
	Dev    string // IB设备名
	Port   string // IB端口
	Index  string // GID Index
	Gid    string // GID值
	Iface  string // 绑定的网络设备名，如 eth0
	Status string // 绑定设备的UP/DOWN状态
}

func CheckRoCEPerfHealth(
	ibBwPerfType string,
	expectedBandwidthGbps, expectedLatencyUs float64,
	ibDevice, netDevice string,
	msgSize int,
	testDuring int,
	gid, qpNum int,
	useGDR, verbose bool,
) (*common.Result, error) {
	var allSpecifiedDevFound bool
	var usedDeviceInfos []string
	status := consts.StatusNormal
	checkRes := make([]*common.CheckerResult, 0)
	var specifiedDevs string
	if netDevice != "" {
		allSpecifiedDevFound, usedDeviceInfos = resolveActiveIBDeviceByNet(netDevice, verbose)
		if !allSpecifiedDevFound {
			specifiedDevs = netDevice
			fmt.Printf("Not all specificd RoCE v2 devices are found with active status, spec: %v, actual: %v\n", netDevice, usedDeviceInfos)
		}
	} else {
		allSpecifiedDevFound, usedDeviceInfos = resolveActiveIBDevice(ibDevice, verbose)
		if !allSpecifiedDevFound && ibDevice != "" {
			specifiedDevs = ibDevice
			fmt.Printf("Not all specificd RoCE v2 devices are found with active status, spec: %v, actual: %v\n", ibDevice, usedDeviceInfos)
		} else if !allSpecifiedDevFound {
			fmt.Println("No active RoCE v2 devices found")
		}
	}

	fmt.Printf("Using RoCE devices for performance test: %v\n", usedDeviceInfos)
	for _, srcDev := range usedDeviceInfos {
		for _, dstDev := range usedDeviceInfos {
			resTemplate := PerfCheckItems[IBPerfTestName]
			resItem := &common.CheckerResult{
				Name:   resTemplate.Name,
				Status: consts.StatusNormal,
			}

			resItem.Detail = fmt.Sprintf("Testing %s -> %s", srcDev, dstDev)

			var metrics float64
			out, err := RunLocalIBTest(ibBwPerfType, srcDev, dstDev, msgSize, testDuring, gid, qpNum, useGDR, verbose)
			if err == nil {
				if strings.Contains(ibBwPerfType, "lat") {
					metrics, err = parseLatency(out, msgSize, srcDev, dstDev)
					if err != nil || metrics > expectedLatencyUs {
						resItem.Status = consts.StatusAbnormal
						resItem.Detail += fmt.Sprintf(" ❌ %.2f us > expected %.2f us", metrics, expectedLatencyUs)
						status = consts.StatusAbnormal
					} else {
						resItem.Status = consts.StatusNormal
						resItem.Detail += fmt.Sprintf(" ✅ %.2f us <= expected %.2f us", metrics, expectedLatencyUs)
					}
				} else {
					metrics, err = parseBandwidth(out, msgSize, srcDev, dstDev)
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

	if len(usedDeviceInfos) == 0 {
		resItem := &common.CheckerResult{
			Name:        IBPerfTestName,
			Description: "No active RoCE devices found to perform bandwidth test",
			Spec:        specifiedDevs,
			Curr:        strings.Join(usedDeviceInfos, ","),
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      fmt.Sprintf("no active RoCE devices found: %v, while %s are specificd ", usedDeviceInfos, specifiedDevs),
			ErrorName:   "DeactiveIBDevicesFound",
			Suggestion:  "Check RoCE device connections and configurations",
		}
		checkRes = append(checkRes, resItem)
		status = consts.StatusAbnormal
	}

	if !allSpecifiedDevFound {
		resItem := &common.CheckerResult{
			Name:        IBPerfTestName,
			Description: "Not all RoCE devices found are active to perform bandwidth test",
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      fmt.Sprintf("Not all RoCE devices found: %v, while %s are specificd ", usedDeviceInfos, specifiedDevs),
			ErrorName:   "DeactiveIBDevicesFound",
			Suggestion:  "Check RoCE device connections and configurations",
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

func resolveActiveIBDevice(ibDev string, verbose bool) (bool, []string) {
	infos, err := GetRoCEv2IPv4DevInfoWithNetStatus()
	if err != nil {
		logrus.WithError(err).Error("Failed to retrieve RoCE v2 device info")
		return false, nil
	}

	var activeDevs []string
	if ibDev == "" {
		for _, info := range infos {
			if info.Status == "up" {
				activeDevs = append(activeDevs, info.Dev)
				if verbose {
					logrus.Infof("Active RoCE v2 device: IBDev=%s GID=%s Iface=%s Status=up", info.Dev, info.Gid, info.Iface)
				}
			} else {
				if verbose {
					logrus.Warnf("RoCE v2 device %s is not up (status: %s)", info.Dev, info.Status)
				}
			}
		}
		if len(activeDevs) == 0 {
			fmt.Println("No active RoCE v2 devices found")
			return false, nil
		}
		return len(activeDevs) > 0, activeDevs
	}

	devFilter := map[string]struct{}{}
	for _, n := range strings.Split(ibDev, ",") {
		devFilter[strings.TrimSpace(n)] = struct{}{}
	}

	for _, info := range infos {
		if _, ok := devFilter[info.Dev]; ok {
			if info.Status != "up" {
				logrus.Warnf("RoCE v2 device %s (%s) is not up (status: %s)", info.Dev, info.Iface, info.Status)
			} else {
				activeDevs = append(activeDevs, info.Dev)
				if verbose {
					logrus.Infof("Active RoCE v2 device: IBDev=%s GID=%s Iface=%s Status=up", info.Dev, info.Gid, info.Iface)
				}
			}
		}
	}

	for dev := range devFilter {
		found := false
		for _, info := range infos {
			if info.Dev == dev {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Specified dev device %s not found in active RoCE devices\n", dev)
			return false, nil
		}
	}

	return len(activeDevs) > 0, activeDevs
}

func resolveActiveIBDeviceByNet(netDev string, verbose bool) (bool, []string) {
	infos, err := GetRoCEv2IPv4DevInfoWithNetStatus()
	if err != nil {
		logrus.WithError(err).Error("Failed to retrieve RoCE v2 device info")
		return false, nil
	}

	var activeDevs []string
	if netDev == "" {
		for _, info := range infos {
			if info.Status == "up" {
				activeDevs = append(activeDevs, info.Dev)
				if verbose {
					logrus.Infof("Valid RoCE v2 device: IBDev=%s GID=%s Iface=%s Status=up", info.Dev, info.Gid, info.Iface)
				}
			} else {
				if verbose {
					logrus.Warnf("RoCE v2 device %s is not up (status: %s)", info.Dev, info.Status)
				}
			}
		}
		if len(activeDevs) == 0 {
			fmt.Println("No valid RoCE v2 devices found")
			return false, nil
		}
		return true, activeDevs
	}

	netFilter := map[string]struct{}{}
	for _, n := range strings.Split(netDev, ",") {
		netFilter[strings.TrimSpace(n)] = struct{}{}
	}

	for _, info := range infos {
		if _, ok := netFilter[info.Iface]; ok {
			if info.Status != "up" {
				fmt.Printf("Net device %s is not up\n", info.Iface)
			} else {
				activeDevs = append(activeDevs, info.Dev)
				if verbose {
					logrus.Infof("Valid RoCE v2 device: IBDev=%s GID=%s Iface=%s Status=up", info.Dev, info.Gid, info.Iface)
				}
			}
		}
	}

	allDevFound := true
	for iface := range netFilter {
		found := false
		for _, info := range infos {
			if info.Iface == iface {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Specified net device %s not found in RoCE devices\n", iface)
			allDevFound = false
		}
	}

	return allDevFound, activeDevs
}

func GetRoCEv2IPv4DevInfoWithNetStatus() (map[string]*RoCEV2DeviceInfo, error) {
	basePath := "/sys/class/infiniband"
	rocev2DevInfo := make(map[string]*RoCEV2DeviceInfo)

	devs, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("Cannot read /sys/class/infiniband: " + err.Error())
	}

	for _, dev := range devs {
		devName := dev.Name()
		portPath := filepath.Join(basePath, devName, "ports")
		ports, err := os.ReadDir(portPath)
		if err != nil {
			continue
		}

		for _, port := range ports {
			portNum := port.Name()
			// Check all GID indexes under this port
			gidDir := filepath.Join(portPath, portNum, "gids")
			gids, err := os.ReadDir(gidDir)
			if err != nil {
				continue
			}

			for _, gidEntry := range gids {
				idx := gidEntry.Name()
				gidFile := filepath.Join(gidDir, idx)
				typeFile := filepath.Join(portPath, portNum, "gid_attrs", "types", idx)

				gidValBytes, err1 := os.ReadFile(gidFile)
				typeValBytes, err2 := os.ReadFile(typeFile)
				if err1 != nil || err2 != nil {
					continue
				}

				gidVal := strings.TrimSpace(string(gidValBytes))
				typeVal := strings.TrimSpace(string(typeValBytes))
				var ipv4MappedGIDPattern = regexp.MustCompile(`^(0000:){5}ffff:`)
				if typeVal == "RoCE v2" && ipv4MappedGIDPattern.MatchString(gidVal) {
					ndevFile := filepath.Join(portPath, portNum, "gid_attrs", "ndevs", idx)
					if _, err := os.Stat(ndevFile); err != nil {
						logrus.WithField("perftest", "rocev2").Warnf("No ndevs file for %s/%s/gids/%s", devName, portNum, idx)
						continue
					}
					ndevBytes, err := os.ReadFile(ndevFile)
					if err != nil {
						logrus.WithField("perftest", "rocev2").Warnf("Cannot read ndevs file for %s/%s/gids/%s: %v", devName, portNum, idx, err)
						continue
					}
					ndev := strings.TrimSpace(string(ndevBytes))
					if ndev == "" {
						logrus.WithField("perftest", "rocev2").Warnf("No network device for %s/%s/gids/%s", devName, portNum, idx)
						continue
					}
					status := getNetDevStatus(ndev)
					logrus.WithField("perftest", "rocev2").Infof("Found RoCE v2 GID %s on device %s port %s index %s with network device %s (status: %s)", gidVal, devName, portNum, idx, ndev, status)
					rocev2DevInfo[devName] = &RoCEV2DeviceInfo{
						Dev:    devName,
						Port:   portNum,
						Index:  idx,
						Gid:    gidVal,
						Iface:  ndev,
						Status: status,
					}
				}
			}
		}
	}
	return rocev2DevInfo, nil
}

func getNetDevStatus(iface string) string {
	if iface == "" {
		return "unknown"
	}
	stateFile := fmt.Sprintf("/sys/class/net/%s/operstate", iface)
	state, err := os.ReadFile(stateFile)
	if err != nil {
		logrus.WithField("perftest", "rocev2").Warnf("Cannot read operstate for %s: %v", iface, err)
		return "unknown"
	}
	return strings.TrimSpace(string(state))
}
