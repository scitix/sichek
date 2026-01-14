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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	IBSYSPathPre    = "/sys/class/infiniband/"
	gatewayCacheTTL = 5 * time.Minute
)

var (
	IBVendorIDs = []string{
		"0x15b3", // Mellanox Technologies
	}
	IBDeviceIDs = []string{
		"0x101b", // MT28908 Family [ConnectX-6]
		"0x101d", // MT28908 Family [ConnectX-6]
		"0x1021", // CMT2910 Family [ConnectX-7]
		"0x1023", // CX8 Family [ConnectX-8]
		"0xa2dc", // BlueField-3 E-series SuperNIC
		"0x09a2", // CMT2910 Family [ConnectX-7] HHHL
		"0x2330", // HPE/Enhance 400G
		"0x4128",
		"0x02b2",
	}
)

func IsModuleLoaded(moduleName string) bool {
	file, err := os.Open("/proc/modules")
	if err != nil {
		fmt.Printf("Unable to open the /proc/modules file: %v\n", err)
		return false
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Error closing file: %v\n", closeErr)
		}
	}()

	return checkModuleInFile(moduleName, file)
}

func checkModuleInFile(moduleName string, file *os.File) bool {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == moduleName {
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("An error occurred while reading the file: %v\n", err)
	}

	return false
}

func ListDir(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		logrus.WithField("component", "infiniband").Infof("Fail to Read dir:%s", dir)
		return nil, err
	}

	fileNames := make([]string, 0, len(files))
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}
	return fileNames, nil
}

func ReadFileLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to open file: %v", err)
		return nil, err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Error closing file: %v\n", closeErr)
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		logrus.WithField("component", "infiniband").Errorf("Error while reading file: %v", err)
		return nil, err
	}
	return lines, nil
}

// GetFileCnt reads content from a path:
//   - if directory, return entry names
//   - if file, return file lines
func GetFileCnt(path string) ([]string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Invalid Path: %v", err)
		return nil, err
	}

	if fileInfo.IsDir() {
		return ListDir(path)
	}
	return ReadFileLines(path)
}

// ReadIBDevSysfileLines gets system content from specified path
func ReadIBDevSysfileLines(IBDev string, DstPath string) ([]string, error) {
	fullPath := path.Join(IBSYSPathPre, IBDev, DstPath)
	return GetFileCnt(fullPath)
}

func GetIBDevBDF(IBDev string) []string {
	ueventInfo, err := ReadIBDevSysfileLines(IBDev, "device/uevent")
	if err != nil || len(ueventInfo) == 0 {
		logrus.WithField("component", "infiniband").Errorf("Failed to read uevent for %s: %v", IBDev, err)
		return nil
	}

	var BDF string
	for j := 0; j < len(ueventInfo); j++ {
		if strings.Contains(ueventInfo[j], "PCI_SLOT_NAME") {
			BDF = strings.Split(ueventInfo[j], "=")[1]
		}
	}
	return []string{BDF}
}

// getBondInterface gets bond interface for a slave interface
func getBondInterface(slaveInterface string) (string, bool) {
	bondPattern := "/sys/class/net/bond*"
	bondDirs, err := filepath.Glob(bondPattern)
	if err != nil {
		return "", false
	}

	for _, bondDir := range bondDirs {
		slavesFile := filepath.Join(bondDir, "bonding/slaves")
		data, err := os.ReadFile(slavesFile)
		if err != nil {
			continue
		}

		slaves := strings.Fields(string(data))
		for _, slave := range slaves {
			if slave == slaveInterface {
				return filepath.Base(bondDir), true
			}
		}
	}

	return "", false
}

// GetIBdev2NetDev returns final network interfaces for an IB device.
// - PF only (VF should already be filtered outside)
// - Bond-aware
func GetIBdev2NetDev(ibDev string) string {
	netPath := filepath.Join("/sys/class/infiniband", ibDev, "device/net")
	physDevs, err := os.ReadDir(netPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to GetIBdev2NetDev for %s: %v", ibDev, err)
		return ""
	}
	if len(physDevs) == 0 {
		logrus.WithField("component", "infiniband").Errorf("no network interface found for IB device %s", ibDev)
		return ""
	}
	physicalIface := physDevs[0].Name()
	if bond, ok := getBondInterface(physicalIface); ok {
		return bond
	}
	return physicalIface
}
