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
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/lldp/collector"
	"github.com/scitix/sichek/components/lldp/config"
	"github.com/scitix/sichek/consts"

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

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	lldpInfo, ok := info.(*collector.LldpInfo)
	if !ok {
		logrus.WithField("component", "lldp").Errorf("invalid data type, expected *LldpInfo")
		return false
	}

	if !lldpInfo.LldpdAvailable {
		fmt.Printf("\n%sLLDP%s: lldpd not available: %s\n\n", consts.Yellow, consts.Reset, lldpInfo.Reason)
		return true
	}
	if len(lldpInfo.Interfaces) == 0 {
		fmt.Printf("\n%sLLDP%s: no neighbors detected\n\n", consts.Yellow, consts.Reset)
		return true
	}

	fmt.Printf("\n%sLLDP neighbors (%d)%s\n", consts.Green, len(lldpInfo.Interfaces), consts.Reset)
	for _, iface := range lldpInfo.Interfaces {
		mgmt := ""
		if len(iface.Neighbor.Chassis.MgmtIP) > 0 {
			mgmt = iface.Neighbor.Chassis.MgmtIP[0]
		}
		ip := ""
		if len(iface.Local.IPv4) > 0 {
			ip = iface.Local.IPv4[0]
		}
		fmt.Printf("  %-18s [%s %s] -> %s/%s @ %s\n",
			iface.Local.Name,
			iface.Local.OperState, ip,
			iface.Neighbor.Chassis.Name, iface.Neighbor.Port.ID, mgmt)
	}
	fmt.Println()
	return true
}
