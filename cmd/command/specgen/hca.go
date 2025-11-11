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
package specgen

import (
	"fmt"

	"github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/components/infiniband/collector"
)

func FillHcaSpec() map[string]config.HCASpec {
	fmt.Println("💽 Configuring HCA spec...")
	specs := map[string]config.HCASpec{}

	// Ask how many HCA devices
	numHcas := promptInt("How many HCA devices to configure?", 1)

	for i := 1; i <= numHcas; i++ {
		fmt.Printf("\n🔧 Configuring HCA device #%d:\n", i)

		// Basic identification
		boardID := promptString(fmt.Sprintf("Board ID for HCA #%d (e.g. MT_0000001070)", i))

		// Hardware information
		fmt.Printf("📋 Hardware Information for %s:\n", boardID)
		hw := collector.IBHardWareInfo{
			BoardID:      boardID,
			HCAType:      promptString("HCA Type (e.g. MT4129, MT4123)", "MT4129"),
			FWVer:        promptString("Firmware version", ">=28.39.2048"),
			PortSpeed:    promptString("Port speed", "400 Gb/sec (4X NDR)"),
			PhyState:     promptString("Physical state", "LinkUp"),
			PortState:    promptString("Port state", "ACTIVE"),
			NetOperstate: promptString("Network operational state", "down"),
			LinkLayer:    promptString("Link layer", "InfiniBand"),
			PCIEWidth:    promptString("PCIe width", "16"),
			PCIESpeed:    promptString("PCIe speed", "32.0 GT/s PCIe"),
			// PCIETreeWidthMin: promptString("PCIe tree width", "32"),
			// PCIETreeSpeedMin: promptString("PCIe tree speed", "16"),
			PCIEMRR: promptString("PCIe MRR", "4096"),
		}

		// Performance information
		fmt.Printf("⚡ Performance Information for %s:\n", boardID)
		perf := config.HCAPerf{
			OneWayBW:   promptFloat("One-way bandwidth (Gbps)", 360),
			AvgLatency: promptFloat("Average latency (us)", 10.0),
		}

		specs[boardID] = config.HCASpec{
			Hardware: hw,
			Perf:     perf,
		}

		fmt.Printf("✅ HCA #%d (%s) configured successfully\n", i, boardID)
	}

	fmt.Printf("\n🎉 Configured %d HCA device(s) successfully\n", numHcas)
	return specs
}
