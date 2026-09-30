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
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/config"

	"github.com/sirupsen/logrus"
)

type OVSCollector struct {
	name string
	spec *config.OVSSpec
}

func NewOVSCollector(spec *config.OVSSpec) *OVSCollector {
	return &OVSCollector{name: "OVSCollector", spec: spec}
}

func (c *OVSCollector) Name() string { return c.name }

// run executes a command and returns trimmed stdout, ignoring non-zero exit (caller checks output).
func run(ctx context.Context, name string, args ...string) string {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		logrus.WithField("component", "ovs").Debugf("cmd %s %v failed: %v", name, args, err)
	}
	return strings.TrimSpace(string(out))
}

func ovsGet(ctx context.Context, args ...string) string {
	return strings.Trim(run(ctx, "ovs-vsctl", args...), `"`)
}

// Collect satisfies common.Collector. On non-DOCA-OVS nodes it returns an
// OVSInfo with Available=false and no error.
func (c *OVSCollector) Collect(ctx context.Context) (common.Info, error) {
	info := &OVSInfo{
		Time:        time.Now(),
		Services:    map[string]string{},
		Packages:    map[string]string{},
		OtherConfig: map[string]string{},
		Coverage:    map[string]uint64{},
	}

	// Gate: ovs-vsctl on PATH + ovs-vswitchd active.
	if _, err := exec.LookPath("ovs-vsctl"); err != nil {
		info.Available = false
		info.SkipReason = "ovs-vsctl not found"
		return info, nil
	}
	for _, svc := range []string{"openvswitch-switch", "ovs-vswitchd", "ovsdb-server"} {
		info.Services[svc] = run(ctx, "systemctl", "is-active", svc)
	}
	if info.Services["ovs-vswitchd"] != "active" {
		info.Available = false
		info.SkipReason = "ovs-vswitchd not active"
		return info, nil
	}
	info.Available = true

	// Packages
	for _, pkg := range c.spec.RequiredPackages {
		info.Packages[pkg] = dpkgVersion(ctx, pkg)
	}

	// Versions + dpdk_initialized + other_config
	info.OVSVersion = ovsGet(ctx, "get", "Open_vSwitch", ".", "ovs_version")
	info.DPDKVersion = ovsGet(ctx, "get", "Open_vSwitch", ".", "dpdk_version")
	info.DPDKInitialized = ovsGet(ctx, "get", "Open_vSwitch", ".", "dpdk_initialized") == "true"
	for k := range c.spec.OtherConfig {
		info.OtherConfig[k] = ovsGet(ctx, "--if-exists", "get", "Open_vSwitch", ".", "other_config:"+k)
	}

	// Bridges. list-br is fetched once and shared across all rails.
	lsOut := run(ctx, "ovs-vsctl", "list-br")
	for r := 0; r < c.spec.NumRails; r++ {
		br := fmt.Sprintf("%s%d", c.spec.BridgePrefix, r)
		info.Bridges = append(info.Bridges, c.collectBridge(ctx, br, lsOut))
	}

	// Datapath / PMD / coverage (info + metrics only)
	info.Datapath = parseDatapath(run(ctx, "ovs-appctl", "dpctl/show"))
	info.Datapath.PMDs = parsePMDPerf(run(ctx, "ovs-appctl", "dpif-netdev/pmd-perf-show"))
	info.Coverage = parseCoverage(run(ctx, "ovs-appctl", "coverage/show"), c.spec.CoverageEvents)

	return info, nil
}

// collectBridge gathers per-bridge state. lsOut is the shared `ovs-vsctl list-br`
// output captured once by Collect; existence is checked against it rather than
// re-running list-br per bridge.
func (c *OVSCollector) collectBridge(ctx context.Context, br, lsOut string) BridgeInfo {
	b := BridgeInfo{Name: br}
	// Detect bridge membership via the shared list-br output (br-exists has no stdout).
	for _, x := range strings.Split(lsOut, "\n") {
		if strings.TrimSpace(x) == br {
			b.Exists = true
			break
		}
	}
	if !b.Exists {
		return b
	}
	b.DatapathType = ovsGet(ctx, "get", "bridge", br, "datapath_type")
	b.FailMode = ovsGet(ctx, "get", "bridge", br, "fail_mode")
	ports := strings.Fields(run(ctx, "ovs-vsctl", "list-ports", br))
	b.Ports = len(ports)
	// dump-flows is run once and reused for both the flow count and the port refs
	// (avoids a duplicate exec + data-consistency skew on a live switch).
	flowsOut := run(ctx, "ovs-ofctl", "dump-flows", br)
	b.Flows = parseFlowCount(flowsOut)
	b.GroupIDs = parseGroupIDs(run(ctx, "ovs-ofctl", "dump-groups", br))
	ofPorts := parseOFShowPorts(run(ctx, "ovs-ofctl", "show", br))
	refs := parseFlowPortRefs(flowsOut)
	b.OrphanFlowRefs, b.OrphanPorts = diffPorts(ofPorts, refs)
	for _, p := range ports {
		pi := PortInfo{Name: p}
		pi.OFPort, _ = strconv.Atoi(ovsGet(ctx, "get", "interface", p, "ofport"))
		pi.AdminState = ovsGet(ctx, "get", "interface", p, "admin_state")
		pi.LinkState = ovsGet(ctx, "get", "interface", p, "link_state")
		if e := ovsGet(ctx, "get", "interface", p, "error"); e != "[]" {
			pi.Error = e
		}
		// Single statistics map call per port (mirrors rdma-doctor port_bytes/port_packets).
		stats := ovsGet(ctx, "get", "interface", p, "statistics")
		pi.RxBytes = parseOVSStatMap(stats, "rx_bytes")
		pi.TxBytes = parseOVSStatMap(stats, "tx_bytes")
		pi.RxErrPkts = parseOVSStatMap(stats, "rx_errors")
		b.PortDetails = append(b.PortDetails, pi)
	}
	return b
}

// dpkgVersion returns the installed version, or "" if not installed (CurrentStatus != i).
func dpkgVersion(ctx context.Context, pkg string) string {
	out := run(ctx, "dpkg-query", "-W", "-f=${db:Status-Abbrev} ${Version}", pkg)
	fields := strings.Fields(out)
	if len(fields) < 2 || len(fields[0]) < 2 || fields[0][1] != 'i' {
		return ""
	}
	return fields[1]
}
