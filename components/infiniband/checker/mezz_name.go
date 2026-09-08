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

package checker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

// defaultIBSysDir is the sysfs root that enumerates InfiniBand devices. Each
// entry directory name is the RDMA device name itself, so the mezz naming check
// needs nothing but this tree — no HCA spec, no heavy collector.
const defaultIBSysDir = "/sys/class/infiniband"

// mezzNameRe is the naming convention rdma-env-pre's interface-naming applies to
// a mezz card's RDMA device: mezz_<k>. A mezz card (board_id config.MezzBoardID)
// whose device name does not match this has not been named, which is the failure
// this checker exists to catch.
var mezzNameRe = regexp.MustCompile(`^mezz_\d+$`)

// IBMezzNameChecker verifies that every mezz card (identified solely by its
// board_id being in config.MezzBoardIDs, per rdma-env-pre
// docs/mezz-card-identification.md) has its RDMA device named per the mezz_<k>
// convention.
//
// It is deliberately spec-free: it reads sysfs directly rather than the collected
// InfinibandInfo, so its verdict is identical whether it runs on the healthy path
// (via common.Check) or is invoked directly from the IB component's initError
// path. That matters because the mezz board_id has no HCA spec by design, which
// otherwise puts the whole IB component into initError where no normal checker runs.
type IBMezzNameChecker struct {
	name string
}

func NewIBMezzNameChecker(_ *config.InfinibandSpec) (common.Checker, error) {
	return &IBMezzNameChecker{name: config.CheckIBMezzName}, nil
}

func (c *IBMezzNameChecker) Name() string { return c.name }

// Check ignores the collected data on purpose and performs its own sysfs scan so
// the healthy path and the initError path share one implementation.
func (c *IBMezzNameChecker) Check(_ context.Context, _ any) (*common.CheckerResult, error) {
	return MezzNamingResult(), nil
}

// MezzNamingResult runs the mezz naming check against the live sysfs tree.
func MezzNamingResult() *common.CheckerResult {
	return mezzNamingResultAt(defaultIBSysDir)
}

// mezzNamingResultAt is the testable core: it scans sysRoot (…/sys/class/infiniband)
// for mezz cards (board_id in config.MezzBoardIDs) and checks each one's device
// directory name against mezz_<k>. Non-mezz devices are ignored; a host with no
// mezz card (or no IB sysfs at all) passes.
func mezzNamingResultAt(sysRoot string) *common.CheckerResult {
	result := config.InfinibandCheckItems[config.CheckIBMezzName]

	entries, err := os.ReadDir(sysRoot)
	if err != nil {
		// No IB sysfs -> nothing to check. Treat as normal so non-IB hosts and
		// hosts without a mezz card do not raise a false Critical.
		result.Status = consts.StatusNormal
		result.Detail = "no infiniband devices found"
		return &result
	}

	var lines []string
	var bad []string
	for _, e := range entries {
		dev := e.Name()
		if !config.MezzBoardIDs[readSysAttr(filepath.Join(sysRoot, dev, "board_id"))] {
			continue // not a mezz card
		}
		netdev := mezzNetdevName(dev) // mezz_0 -> mezz0 (display only, not validated)
		link := mezzLinkLabel(sysRoot, dev)
		if mezzNameRe.MatchString(dev) {
			lines = append(lines, fmt.Sprintf("%s port 1 ==> %s (%s)", dev, netdev, link))
		} else {
			lines = append(lines, fmt.Sprintf("%s port 1 ==> %s (%s)  [expected mezz_<k>]", dev, netdev, link))
			bad = append(bad, dev)
		}
	}

	sort.Strings(lines)
	if len(bad) > 0 {
		sort.Strings(bad)
		result.Status = consts.StatusAbnormal
		result.Device = strings.Join(bad, ",")
		result.Detail = strings.Join(lines, "\n")
		return &result
	}

	result.Status = consts.StatusNormal
	if len(lines) == 0 {
		result.Detail = "no mezz card found"
	} else {
		result.Detail = strings.Join(lines, "\n")
	}
	return &result
}

// readSysAttr reads a sysfs attribute file and trims trailing whitespace/newline.
// A read error yields "" so callers can compare against expected values simply.
func readSysAttr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// mezzNetdevName derives the netdev display name (mezz<k>) from the RDMA device
// name (mezz_<k>). Display only — the checker validates the RDMA name, not this.
// A non-conforming input is returned unchanged.
func mezzNetdevName(rdmaDev string) string {
	return strings.Replace(rdmaDev, "mezz_", "mezz", 1)
}

// mezzLinkLabel reads the device's port-1 phys_state and reduces it to Up/Down.
// sysfs phys_state reads like "5: LinkUp" / "2: Polling" / "3: Disabled"; anything
// that is not LinkUp is reported as Down.
func mezzLinkLabel(sysRoot, dev string) string {
	state := readSysAttr(filepath.Join(sysRoot, dev, "ports", "1", "phys_state"))
	if strings.Contains(state, "LinkUp") {
		return "Up"
	}
	return "Down"
}
