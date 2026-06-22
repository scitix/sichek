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
package lldp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/lldp/collector"
	"github.com/scitix/sichek/components/lldp/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.LldpUserConfig
	cfgMutex sync.Mutex

	collector common.Collector

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	lldpComponent     *component
	lldpComponentOnce sync.Once
)

// NewComponent constructs (or returns the previously-constructed) lldp
// component. specFile is ignored — there is no hardware spec for lldp.
func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	lldpComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component lldp: %v", r)
			}
		}()
		lldpComponent, err = newComponent(cfgFile)
	})
	return lldpComponent, err
}

func newComponent(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.LldpUserConfig{}
	if loadErr := common.LoadUserConfig(cfgFile, cfg); loadErr != nil {
		logrus.WithField("component", "lldp").Warnf("load user config failed, using defaults: %v", loadErr)
	}
	if cfg.LLDP == nil {
		cfg.LLDP = &config.LldpConfig{}
	}
	if cfg.LLDP.CacheSize <= 0 {
		cfg.LLDP.CacheSize = 5
	}

	collectorPointer := collector.NewCollector(cfg.LLDP.LldpctlPath, cfg.LLDP.ExecTimeout.Duration)

	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameLLDP,
		collector:     collectorPointer,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cfg.LLDP.CacheSize),
		cacheInfo:     make([]common.Info, cfg.LLDP.CacheSize),
		cacheSize:     cfg.LLDP.CacheSize,
	}
	comp.service = common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	return
}

func (c *component) Name() string { return c.componentName }

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "lldp").Errorf("collect failed: %v", err)
		return nil, err
	}

	// lldp has no health semantics for now — it is purely informational.
	// A future revision can add checkers (e.g. "production NIC has no
	// neighbor") by replacing this stub with common.Check(...).
	result := &common.Result{
		Item:   c.componentName,
		Status: consts.StatusNormal,
		Level:  consts.LevelInfo,
		Time:   time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfo, nil
}

func (c *component) LastInfo() (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	var info common.Info
	if c.currIndex == 0 {
		info = c.cacheInfo[c.cacheSize-1]
	} else {
		info = c.cacheInfo[c.currIndex-1]
	}
	return info, nil
}

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

func (c *component) Start() <-chan *common.Result { return c.service.Start() }

func (c *component) Stop() error { return c.service.Stop() }

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.LldpUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("update wrong config type for lldp")
	}
	c.cfg = configPointer
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) Status() bool { return c.service.Status() }

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval().Duration
}

// lldpRowFmt lays out the neighbor table. Widths are byte-based (the %-Ns
// verb pads bytes), so cell values are kept ASCII and truncated to fit.
const lldpRowFmt = "%-20s %-5s %-19s %-6s %-16s %-18s %-16s %-22s %-6s %-14s %-6s %-9s\n"

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	lldpInfo, ok := info.(*collector.LldpInfo)
	if !ok {
		logrus.WithField("component", "lldp").Errorf("invalid data type, expected *LldpInfo")
		return false
	}

	utils.PrintTitle("LLDP", "-")

	if !lldpInfo.LldpdAvailable {
		fmt.Printf("%slldpd not available%s: %s\n\n", consts.Yellow, consts.Reset, lldpInfo.Reason)
		return true
	}
	if len(lldpInfo.Interfaces) == 0 {
		fmt.Printf("%sno LLDP neighbors detected%s\n\n", consts.Yellow, consts.Reset)
		return true
	}

	hostname, _ := os.Hostname()

	// Separate real switch uplinks from self/loopback and host neighbors.
	// A switch advertises its port as an "ifname"; OVS VF representors loop
	// back to our own hostname and host-to-host neighbors identify their port
	// by MAC. The latter are not switch uplinks, so they are folded away.
	var uplinks []collector.IfaceInfo
	var folded []string
	for _, iface := range lldpInfo.Interfaces {
		if isSwitchUplink(iface, hostname) {
			uplinks = append(uplinks, iface)
		} else {
			folded = append(folded, iface.Local.Name)
		}
	}

	fmt.Printf("%sLLDP neighbors: %d switch uplink(s)%s\n", consts.Green, len(uplinks), consts.Reset)
	fmt.Printf(lldpRowFmt, "Local Port", "Link", "Local IP", "MTU", "Neighbor Switch",
		"Chassis ID", "Mgmt IP", "Remote Port", "VLAN", "Capability", "MFS", "Age")
	fmt.Printf(lldpRowFmt, dashes(20), dashes(5), dashes(19), dashes(6), dashes(16),
		dashes(18), dashes(16), dashes(22), dashes(6), dashes(14), dashes(6), dashes(9))

	for _, iface := range uplinks {
		l := iface.Local
		n := iface.Neighbor
		local := l.Name
		if l.Master != "" {
			local = fmt.Sprintf("%s(%s)", l.Name, l.Master)
		}
		fmt.Printf(lldpRowFmt,
			clip(local, 20),
			orDash(l.OperState),
			clip(firstOrEmpty(l.IPv4), 19),
			intOrDash(l.MTU),
			clip(orDash(n.Chassis.Name), 16),
			clip(orDash(n.Chassis.ID), 18),
			clip(firstOrEmpty(n.Chassis.MgmtIP), 16),
			clip(orDash(n.Port.ID), 22),
			vlanCell(n),
			clip(joinOrDash(n.Chassis.Capability), 14),
			intOrDash(n.Port.MFS),
			fmtAge(n.AgeSeconds),
		)
	}

	if len(folded) > 0 {
		fmt.Printf("\n%d local/loopback or host neighbor(s) hidden: %s\n",
			len(folded), strings.Join(folded, ", "))
	}
	fmt.Println()
	return true
}

// isSwitchUplink reports whether an LLDP neighbor is a real switch uplink
// (as opposed to a self VF-representor loopback or a host-to-host link).
func isSwitchUplink(iface collector.IfaceInfo, hostname string) bool {
	return collector.IsSwitchNeighbor(iface.Neighbor, hostname)
}

func dashes(n int) string { return strings.Repeat("-", n) }

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func firstOrEmpty(ss []string) string {
	if len(ss) == 0 {
		return "-"
	}
	return ss[0]
}

func joinOrDash(ss []string) string {
	if len(ss) == 0 {
		return "-"
	}
	return strings.Join(ss, ",")
}

func intOrDash(i int) string {
	if i <= 0 {
		return "-"
	}
	return strconv.Itoa(i)
}

func vlanCell(n collector.Neighbor) string {
	if n.VlanID == 0 {
		return "-"
	}
	if n.VlanPVID {
		return strconv.Itoa(n.VlanID) + "*"
	}
	return strconv.Itoa(n.VlanID)
}

// fmtAge renders a neighbor age in a compact two-unit form (e.g. "182d0h").
func fmtAge(s int64) string {
	if s <= 0 {
		return "-"
	}
	d := s / 86400
	h := (s % 86400) / 3600
	m := (s % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd%dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// clip truncates s to max bytes, marking truncation with a trailing '+' so
// columns stay aligned under the byte-based %-Ns padding.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "+"
}
