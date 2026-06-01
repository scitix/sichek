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
	"os/exec"
	"sort"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/vishvananda/netlink"
)

const defaultLldpctlPath = "lldpctl"

// LldpInfo is the snapshot payload for the lldp component.
type LldpInfo struct {
	Time           time.Time   `json:"time"`
	LldpdAvailable bool        `json:"lldpd_available"`
	Reason         string      `json:"reason,omitempty"`
	Interfaces     []IfaceInfo `json:"interfaces"`
}

func (i *LldpInfo) JSON() (string, error) {
	b, err := json.Marshal(i)
	return string(b), err
}

// IfaceInfo couples one local iface with its observed LLDP neighbor.
type IfaceInfo struct {
	Local    LocalIface `json:"local"`
	Neighbor Neighbor   `json:"neighbor"`
}

// LocalIface describes the host-side of one LLDP-bearing interface.
type LocalIface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	MTU       int      `json:"mtu,omitempty"`
	OperState string   `json:"oper_state,omitempty"`
	IPv4      []string `json:"ipv4,omitempty"`
	Master    string   `json:"master,omitempty"`
	VlanID    int      `json:"vlan_id,omitempty"`
}

// Collector implements common.Collector for the lldp component.
type Collector struct {
	name        string
	lldpctlPath string
	execTimeout time.Duration
}

// NewCollector returns a Collector. If lldpctlPath is empty, the default
// "lldpctl" name is used and resolved against $PATH at Collect time.
func NewCollector(lldpctlPath string, execTimeout time.Duration) *Collector {
	if lldpctlPath == "" {
		lldpctlPath = defaultLldpctlPath
	}
	if execTimeout <= 0 {
		execTimeout = 10 * time.Second
	}
	return &Collector{
		name:        "LldpCollector",
		lldpctlPath: lldpctlPath,
		execTimeout: execTimeout,
	}
}

func (c *Collector) Name() string { return c.name }

func (c *Collector) Collect(ctx context.Context) (common.Info, error) {
	info := &LldpInfo{
		Time:       time.Now(),
		Interfaces: []IfaceInfo{},
	}

	// 1. Resolve lldpctl binary. Missing lldpctl is not an error — the
	//    snapshot just records lldpd_available=false so downstream
	//    consumers can tell "lldpd not deployed" apart from "no neighbors".
	bin, err := exec.LookPath(c.lldpctlPath)
	if err != nil {
		info.LldpdAvailable = false
		info.Reason = fmt.Sprintf("lldpctl not found: %v", err)
		return info, nil
	}

	// 2. Run lldpctl with a bounded timeout independent of the caller's ctx
	//    so a hung lldpd can't wedge the daemon's ticker.
	runCtx, cancel := context.WithTimeout(ctx, c.execTimeout)
	defer cancel()
	out, err := exec.CommandContext(runCtx, bin, "-f", "json").Output()
	if err != nil {
		info.LldpdAvailable = false
		info.Reason = fmt.Sprintf("lldpctl exec failed: %v", lldpctlErrDetail(err))
		return info, nil
	}
	info.LldpdAvailable = true

	// 3. Parse and enrich.
	neighbors, err := ParseLldpctlJSON(out)
	if err != nil {
		info.Reason = fmt.Sprintf("parse lldpctl output failed: %v", err)
		return info, nil
	}

	names := make([]string, 0, len(neighbors))
	for n := range neighbors {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		info.Interfaces = append(info.Interfaces, IfaceInfo{
			Local:    collectLocalIface(n),
			Neighbor: neighbors[n],
		})
	}
	return info, nil
}

// lldpctlErrDetail extracts stderr from an ExitError so the snapshot's
// Reason string is useful for debugging.
func lldpctlErrDetail(err error) string {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Sprintf("%s: %s", err, ee.Stderr)
	}
	return err.Error()
}

// collectLocalIface reads attributes for the local end of an LLDP link via
// netlink. All errors are swallowed — partial info is better than no info,
// and missing fields stay as zero values in the snapshot.
func collectLocalIface(name string) LocalIface {
	li := LocalIface{Name: name}
	link, err := netlink.LinkByName(name)
	if err != nil {
		return li
	}
	attrs := link.Attrs()
	if attrs == nil {
		return li
	}
	if attrs.HardwareAddr != nil {
		li.MAC = attrs.HardwareAddr.String()
	}
	li.MTU = attrs.MTU
	li.OperState = attrs.OperState.String()
	if attrs.MasterIndex > 0 {
		if master, err := netlink.LinkByIndex(attrs.MasterIndex); err == nil && master != nil && master.Attrs() != nil {
			li.Master = master.Attrs().Name
		}
	}
	if vlan, ok := link.(*netlink.Vlan); ok {
		li.VlanID = vlan.VlanId
	}

	if addrs, err := netlink.AddrList(link, netlink.FAMILY_V4); err == nil {
		for _, a := range addrs {
			if a.IPNet != nil {
				li.IPv4 = append(li.IPv4, a.IPNet.String())
			}
		}
	}
	return li
}
