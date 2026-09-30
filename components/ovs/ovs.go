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
package ovs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/checker"
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	ovsmetrics "github.com/scitix/sichek/components/ovs/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.OVSUserConfig
	cfgMutex      sync.Mutex
	collector     *collector.OVSCollector
	checkers      []common.Checker
	metrics       *ovsmetrics.OVSMetrics

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	ovsComponent     *component
	ovsComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	var err error
	ovsComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component ovs: %v", r)
			}
		}()
		ovsComponent, err = newComponent(cfgFile, specFile, ignoredCheckers)
	})
	return ovsComponent, err
}

func newComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.OVSUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.OVS == nil {
		logrus.WithField("component", "ovs").Warnf("get user config failed or ovs config is nil, using default config")
		cfg.OVS = &config.OVSConfig{
			QueryInterval: common.Duration{Duration: 60 * time.Second},
			CacheSize:     5,
			EnableMetrics: true,
		}
	}
	if len(ignoredCheckers) > 0 {
		cfg.OVS.IgnoredCheckers = ignoredCheckers
	}

	spec, err := config.LoadSpec(specFile)
	if err != nil {
		logrus.WithField("component", "ovs").Warnf("failed to load spec %s: %v", specFile, err)
	}

	checkers, err := checker.NewCheckers(cfg, spec)
	if err != nil {
		return nil, err
	}

	cacheSize := cfg.OVS.CacheSize
	if cacheSize == 0 {
		cacheSize = 5
	}

	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameOVS,
		collector:     collector.NewOVSCollector(spec),
		checkers:      checkers,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cacheSize),
		cacheInfo:     make([]common.Info, cacheSize),
		cacheSize:     cacheSize,
		metrics:       ovsmetrics.NewOVSMetrics(),
	}
	comp.service = common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	return comp, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "ovs").Errorf("failed to collect ovs info: %v", err)
		return nil, err
	}
	ovsInfo, ok := info.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("wrong ovs collector info type")
	}

	if c.cfg.OVS != nil && c.cfg.OVS.EnableMetrics {
		c.metrics.ExportMetrics(ovsInfo)
	}

	var result *common.Result
	if !ovsInfo.Available {
		// Graceful no-op on non-DOCA-OVS nodes: one info checker, no issues.
		result = &common.Result{
			Item: c.componentName, Status: consts.StatusNormal, Level: consts.LevelInfo,
			Time: time.Now(),
			Checkers: []*common.CheckerResult{{
				Name: "ovs_present", Description: "DOCA-OVS availability",
				Status: consts.StatusNormal, Level: consts.LevelInfo,
				Curr: "skipped", Detail: ovsInfo.SkipReason,
			}},
		}
	} else {
		result = common.Check(ctx, c.componentName, ovsInfo, c.checkers)
	}

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = ovsInfo
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
	if c.currIndex == 0 {
		return c.cacheInfo[c.cacheSize-1], nil
	}
	return c.cacheInfo[c.currIndex-1], nil
}

func (c *component) Start() <-chan *common.Result { return c.service.Start() }
func (c *component) Stop() error                  { return c.service.Stop() }
func (c *component) Status() bool                 { return c.service.Status() }
func (c *component) GetTimeout() time.Duration    { return c.cfg.GetQueryInterval().Duration }

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	cp, ok := cfg.(*config.OVSUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("update wrong config type for ovs")
	}
	c.cfg = cp
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := !(result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo])
	utils.PrintTitle("OVS", "-")
	ovsInfo, ok := info.(*collector.OVSInfo)
	if !ok || ovsInfo == nil {
		fmt.Println("No OVS info available")
		return checkAllPassed
	}
	if !ovsInfo.Available {
		fmt.Printf("OVS not active on this node: %s\n", ovsInfo.SkipReason)
		return checkAllPassed
	}
	fmt.Printf("OVS %s / DPDK %s  dpdk_initialized=%v\n", ovsInfo.OVSVersion, ovsInfo.DPDKVersion, ovsInfo.DPDKInitialized)
	fmt.Printf("%-12s %-9s %-7s %-7s %s\n", "bridge", "datapath", "ports", "flows", "groups")
	for _, b := range ovsInfo.Bridges {
		fmt.Printf("%-12s %-9s %-7d %-7d %d\n", b.Name, b.DatapathType, b.Ports, b.Flows, len(b.GroupIDs))
	}
	if result != nil {
		for _, res := range result.Checkers {
			if res.Status != consts.StatusNormal && res.Level != consts.LevelInfo {
				fmt.Printf("\tEvent: %s%s%s -> %s\n", consts.LevelColor(res.Level), res.ErrorName, consts.Reset, res.Detail)
			}
		}
	}
	fmt.Println()
	return checkAllPassed
}
