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
package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type EthernetInfo struct {
	EthDevs         []string          `json:"eth_dev"`
	EthHardWareInfo []EthHardWareInfo `json:"eth_hardware_info"`
	Time            time.Time         `json:"time"`
}

type EthHardWareInfo struct {
	EthDev  string `json:"eth_dev"`
	PhyStat string `json:"phy_stat"`
}

var (
	EthSYSPathPre string = "/sys/class/net/"
)

func (i *EthernetInfo) GetPhyState(ethDev string) []string {
	return i.GetSysCnt(ethDev, "operstate")
}

func (i *EthernetInfo) GetSysCnt(ethDev string, DstPath string) []string {
	var allCnt []string
	DesPath := path.Join(EthSYSPathPre, ethDev, DstPath)
	Cnt := GetFileCnt(DesPath)
	allCnt = append(allCnt, Cnt...)
	return allCnt
}

func (i *EthernetInfo) JSON() (string, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (i *EthernetInfo) isVirtualInterface(iface string) bool {
	virtualPrefixes := []string{"veth", "docker", "virbr", "br-", "tunl", "flannel", "tap", "eno"}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(iface, prefix) {
			return true
		}
	}

	_, err := os.Stat(fmt.Sprintf("/sys/class/net/%s/device", iface))
	return os.IsNotExist(err)
}

func (i *EthernetInfo) getInterfaceSpeed(iface string) (int, error) {
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", iface)
	data, err := os.ReadFile(speedPath)
	if err != nil {
		return 0, err
	}

	speedStr := strings.TrimSpace(string(data))
	speed, err := strconv.Atoi(speedStr)
	if err != nil {
		return 0, err
	}

	return speed, nil
}

func (i *EthernetInfo) GetEthDevs() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("Error retrieving network interfaces: %v\n", err)
		return nil
	}

	var mgtIface []string
	for _, iface := range interfaces {
		if i.isVirtualInterface(iface.Name) {
			continue
		}
		logrus.WithField("component", "ethetnet").Infof("get the phy interface %v", iface)
		speed, err := i.getInterfaceSpeed(iface.Name)
		if err != nil {
			logrus.WithField("component", "ethetnet").Errorf("Interface %s: Unable to determine speed: %v\n", iface.Name, err)
			continue
		}

		if speed == 10000 || speed == 25000 {
			logrus.WithField("component", "ethetnet").Infof("Interface %s is a physical network interface with speed %dG.\n", iface.Name, speed/1000)
			mgtIface = append(mgtIface, iface.Name)
		}
	}
	return mgtIface
}

func (i *EthernetInfo) GetIBInfo() *EthernetInfo {
	var ethInfo EthernetInfo
	ethInfo.EthDevs = ethInfo.GetEthDevs()

	allEthHWInfo := make([]EthHardWareInfo, len(ethInfo.EthDevs))
	for _, EthDev := range ethInfo.EthDevs {
		var perEthHWInfo EthHardWareInfo
		perEthHWInfo.EthDev = EthDev
		perEthHWInfo.PhyStat = i.GetPhyState(EthDev)[0]
		perEthHWInfo.EthDev = EthDev
		ethInfo.EthDevs = append(ethInfo.EthDevs, EthDev)
		allEthHWInfo = append(allEthHWInfo, perEthHWInfo)
	}

	ethInfo.EthHardWareInfo = allEthHWInfo
	ethInfo.Time = time.Now()
	return &ethInfo
}

func GetFileCnt(path string) []string {
	fileInfo, err := os.Stat(path)
	if err != nil {
		logrus.WithField("component", "ethetnet").Errorf("Invalid Path: %v", err)
		return nil
	}

	var results []string
	if fileInfo.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			logrus.WithField("component", "ethetnet").Errorf("Failed to read directory: %v", err)
			return nil
		}

		for _, file := range files {
			results = append(results, file.Name())
		}
	} else {
		results = readFileContent(path)
	}
	return results
}
func readFileContent(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithField("component", "ethetnet").Errorf("Failed to open file: %v", err)
		return nil
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logrus.WithField("component", "ethetnet").Errorf("Error closing file: %v\n", closeErr)
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		logrus.WithField("component", "ethetnet").Errorf("Error while reading file: %v", err)
	}
	return lines
}
