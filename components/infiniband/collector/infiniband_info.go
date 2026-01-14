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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/sirupsen/logrus"
)

type InfinibandInfo struct {
	HCAPCINum       int                       `json:"hca_pci_num" yaml:"hca_pci_num"`
	IBCapablePCINum int                       `json:"ib_capable_pci_num" yaml:"ib_capable_pci_num"`
	IBPFDevs        map[string]string         `json:"ib_dev" yaml:"ib_dev"`
	IBPCIDevs       map[string]string         `json:"hca_pci_dev" yaml:"hca_pci_dev"`
	IBHardWareInfo  map[string]IBHardWareInfo `json:"ib_hardware_info" yaml:"ib_hardware_info"`
	IBSoftWareInfo  IBSoftWareInfo            `json:"ib_software_info" yaml:"ib_software_info"`
	// PCIETreeInfo   map[string]PCIETreeInfo   `json:"pcie_tree_info" yaml:"pcie_tree_info"`
	IBCounters map[string]IBCounters `json:"ib_counters" yaml:"ib_counters"`
	IBNicRole  string                `json:"ib_nic_role" yaml:"ib_nic_role"`
	Time       time.Time             `json:"time" yaml:"time"`
	mu         sync.RWMutex
}

func NewIBCollector(ctx context.Context) (*InfinibandInfo, error) {
	i := &InfinibandInfo{
		IBHardWareInfo: make(map[string]IBHardWareInfo),
		IBSoftWareInfo: IBSoftWareInfo{},
		// PCIETreeInfo:   make(map[string]PCIETreeInfo),
		IBPFDevs:   make(map[string]string),
		IBCounters: make(map[string]IBCounters),
		mu:         sync.RWMutex{},
	}
	i.IBNicRole = i.GetNICRole()
	var err error
	// Get PCIe device list at collector initialization
	i.IBPCIDevs, err = GetRDMACapablePCIeDevices()
	i.IBCapablePCINum = len(i.IBPCIDevs)
	if err != nil {
		logrus.WithField("component", "infiniband").Warnf("Failed to find PCI devices: %v", err)
	}

	return i, nil
}

func (i *InfinibandInfo) JSON() (string, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (i *InfinibandInfo) Name() string {
	return "IBcollector"
}

func (i *InfinibandInfo) RLock() {
	i.mu.RLock()
}

func (i *InfinibandInfo) RUnlock() {
	i.mu.RUnlock()
}

func (i *InfinibandInfo) Lock() {
	i.mu.Lock()
}

func (i *InfinibandInfo) Unlock() {
	i.mu.Unlock()
}

func (i *InfinibandInfo) Collect(ctx context.Context) (common.Info, error) {
	i.IBPFDevs = i.GetIBPFdevs()
	i.HCAPCINum = countHCAPCINum(i.IBPFDevs)
	i.IBSoftWareInfo.Collect(ctx)

	// // IBPFDevs is the list of IB PF devices, ignoring cx4 and virtual functions and bond devices
	for IBDev := range i.IBPFDevs {
		// skip mezzanine card
		if strings.Contains(IBDev, "mezz") {
			continue
		}

		// ?? why need to check if the IB device has net or not? is it a secondary port?
		bdfList := GetIBDevBDF(IBDev)
		if len(bdfList) > 0 {
			bdf := bdfList[0]
			if len(bdf) > 0 && strings.HasSuffix(bdf, ".1") {
				netDir := fmt.Sprintf("/sys/bus/pci/devices/%s/net", bdf)
				files, err := os.ReadDir(netDir)
				if err != nil {
					logrus.WithField("component", "infiniband").Errorf("Error reading net dir (driver loaded?): %v", err)
					continue
				}

				if len(files) == 0 {
					logrus.WithField("component", "infiniband").Errorf("No network interface found for this BDF: %s", bdf)
					continue
				}
				masterPath := fmt.Sprintf("/sys/class/net/%s/master", files[0].Name())
				_, err = os.Lstat(masterPath)
				if os.IsNotExist(err) {
					continue
				}
			}
		}

		var hwInfo IBHardWareInfo
		hwInfo.Collect(ctx, IBDev, i.IBNicRole)
		i.IBHardWareInfo[IBDev] = hwInfo

		var counters IBCounters = make(map[string]uint64)
		counters.Collect(IBDev)
		i.IBCounters[IBDev] = counters
	}

	i.Time = time.Now()
	return i, nil
}

func countHCAPCINum(ibPFDevs map[string]string) int {
	pciNum := 0
	// // ibPFDevs is the list of IB PF devices, ignoring cx4 and virtual functions and bond devices
	for ibDev := range ibPFDevs {
		_, isBond := GetIBdev2NetDev(ibDev)
		if isBond {
			pciNum += 2
		} else {
			pciNum += 1
		}
	}

	return pciNum
}

// TODO: Why should it be ignored??? Make it configurable
// var ignoredHCATypes = []string{
// 	"MT4117", // CX4
// 	"MT4119",
// }

// func ignoreByHCATYPE(ibDev string) bool {
// 	hcaTypePath := path.Join(IBSYSPathPre, ibDev, "hca_type")

// 	content, err := os.ReadFile(hcaTypePath)
// 	if err != nil {
// 		logrus.WithField("component", "infiniband").
// 			Warnf("ignoring IBDev %s: failed to read hca_type (%v)", ibDev, err)
// 		return true
// 	}

// 	hcaType := strings.TrimSpace(string(content))

// 	for _, t := range ignoredHCATypes {
// 		if strings.Contains(hcaType, t) {
// 			logrus.WithField("component", "infiniband").
// 				Infof("ignoring IBDev %s due to HCA type: %s", ibDev, hcaType)
// 			return true
// 		}
// 	}

// 	return false
// }

func ignoreVirtualFunction(ibDev string) bool {
	vfPath := path.Join(IBSYSPathPre, ibDev, "device", "physfn")
	if _, err := os.Stat(vfPath); err == nil {
		logrus.WithField("component", "infiniband").
			Infof("ignoring virtual function IBDev: %s", ibDev)
		return true
	}
	return false
}

func (i *InfinibandInfo) GetPFDevs(IBDevs []string) []string {
	PFDevs := make([]string, 0)
	for _, IBDev := range IBDevs {

		// // ignore cx4 interface: why???
		// shouldIgnore := ignoreByHCATYPE(IBDev)
		// if shouldIgnore {
		// 	continue
		// }

		// ignore virtual functions
		shouldIgnore := ignoreVirtualFunction(IBDev)
		if shouldIgnore {
			continue
		}

		PFDevs = append(PFDevs, IBDev)
	}
	return PFDevs
}

// GetIBPFdevs Get IB PF devices igoring virtual functions and bond devices
func (i *InfinibandInfo) GetIBPFdevs() map[string]string {
	allIBDevs, err := GetFileCnt(IBSYSPathPre)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read IB devices directory: %v", err)
		return make(map[string]string)
	}
	PFDevs := i.GetPFDevs(allIBDevs)

	IBPFDevs := make(map[string]string)
	for _, IBDev := range PFDevs {
		// if isIBBondDev(IBDev) {
		// 	continue
		// }
		ibNetDev, _ := GetIBdev2NetDev(IBDev)
		IBPFDevs[IBDev] = ibNetDev
	}
	logrus.WithField("component", "infiniband").Debugf("get the IB and net map: %v", IBPFDevs)

	return IBPFDevs
}

func (i *InfinibandInfo) GetNICRole() string {
	var nodeState string

	cmd := exec.Command("rdma", "system")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "ErrNode"
	}
	outputStr := string(output)
	if strings.Contains(outputStr, "exclusive") {
		nodeState = "sriovNode"
	}

	if strings.Contains(outputStr, "share") {
		nodeState = "macvlanNode"
	}

	return nodeState
}
