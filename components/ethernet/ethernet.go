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
package ethernet

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	filter "github.com/scitix/sichek/components/common/eventfilter"
	"github.com/scitix/sichek/components/ethernet/checker"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.EthernetUserConfig
	cfgMutex      sync.Mutex
	collector     *collector.EthernetCollector
	checkers      []common.Checker
	filter        *filter.EventFilter

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	ethernetComponent     *component
	ethernetComponentOnce sync.Once
)

func NewEthernetComponent(cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	var err error
	ethernetComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component ethernet: %v", r)
			}
		}()
		ethernetComponent, err = newEthernetComponent(cfgFile, specFile, ignoredCheckers)
	})
	return ethernetComponent, err
}

func newEthernetComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.EthernetUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Ethernet == nil {
		logrus.WithField("component", "ethernet").Warnf("get user config failed or ethernet config is nil, using default config")
		cfg.Ethernet = &config.EthernetConfig{
			QueryInterval: common.Duration{Duration: 60 * time.Second},
			CacheSize:     5,
		}
	}
	if len(ignoredCheckers) > 0 {
		cfg.Ethernet.IgnoredCheckers = ignoredCheckers
	}

	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "ethernet").Warnf("failed to load eventrules: %v", err)
	}

	filterPointer, err := filter.NewEventFilter(consts.ComponentNameEthernet, eventRules, 100)
	if err != nil {
		logrus.WithField("component", "ethernet").Warnf("NewEthernetComponent create event filter failed: %v", err)
		filterPointer = nil
	}

	spec, err := config.LoadSpec(specFile)
	if err != nil {
		logrus.WithField("component", "ethernet").Warnf("failed to load spec %s: %v", specFile, err)
	}

	targetBond := ""
	if spec != nil {
		targetBond = spec.TargetBond
	}

	collectorInst, err := collector.NewEthernetCollector(targetBond)
	if err != nil {
		logrus.WithField("component", "ethernet").Errorf("NewEthernetComponent create collector failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(cfg, spec)
	if err != nil {
		return nil, err
	}

	cacheSize := cfg.Ethernet.CacheSize
	if cacheSize == 0 {
		cacheSize = 5
	}

	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameEthernet,
		collector:     collectorInst,
		checkers:      checkers,
		filter:        filterPointer,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cacheSize),
		cacheInfo:     make([]common.Info, cacheSize),
		cacheSize:     cacheSize,
	}
	service := common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	component.service = service

	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	timer := common.NewTimer(fmt.Sprintf("%s-HealthCheck-Cost", c.componentName))
	ethInfo, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "ethernet").Errorf("failed to collect ethernet info: %v", err)
		return nil, err
	}
	logrus.WithField("component", "ethernet").Infof("collected ethernet info: %+v", ethInfo)

	result := common.Check(ctx, c.componentName, ethInfo, c.checkers)
	timer.Mark("ethernet-check")

	if c.filter != nil {
		eventResult := c.filter.Check()
		timer.Mark("event-filter")
		if eventResult != nil {
			result.Checkers = append(result.Checkers, eventResult.Checkers...)
			if eventResult.Status == consts.StatusAbnormal {
				result.Status = consts.StatusAbnormal
				if consts.LevelPriority[result.Level] < consts.LevelPriority[eventResult.Level] {
					result.Level = eventResult.Level
				}
			}
		}
	}

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = ethInfo
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo] {
		logrus.WithField("component", "ethernet").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "ethernet").Infof("Health Check PASSED")
	}

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

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.EthernetUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for ethernet")
	}
	c.cfg = configPointer
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval().Duration
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	if result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo] {
		checkAllPassed = false
	}
	ethEvent := make(map[string]string)

	l1Print := fmt.Sprintf("L1(Link): %sNot Checked%s", consts.Yellow, consts.Reset)
	l2Print := fmt.Sprintf("L2(Bond): %sNot Checked%s", consts.Yellow, consts.Reset)
	l3Print := fmt.Sprintf("L3(LACP): %sNot Checked%s", consts.Yellow, consts.Reset)
	l4Print := fmt.Sprintf("L4(ARP) : %sNot Checked%s", consts.Yellow, consts.Reset)
	l5Print := fmt.Sprintf("L5(Route): %sNot Checked%s", consts.Yellow, consts.Reset)

	utils.PrintTitle("Ethernet", "-")
	checkerResults := result.Checkers
	for _, res := range checkerResults {
		if res.Status != consts.StatusNormal && res.Level != consts.LevelInfo {
			checkAllPassed = false
			ethEvent[res.Name] = fmt.Sprintf("Event: %s%s%s -> %s", consts.Red, res.ErrorName, consts.Reset, strings.TrimRight(res.Detail, "\n"))
		}

		statusColor := consts.Green
		statusText := "OK"
		if res.Status != consts.StatusNormal {
			statusColor = consts.Red
			statusText = "Err"
		}

		switch res.Name {
		case config.EthernetL1CheckerName:
			l1Print = fmt.Sprintf("L1(Physical Link): %s%s%s", statusColor, statusText, consts.Reset)
		case config.EthernetL2CheckerName:
			l2Print = fmt.Sprintf("L2(Bonding)      : %s%s%s", statusColor, statusText, consts.Reset)
		case config.EthernetL3CheckerName:
			l3Print = fmt.Sprintf("L3(LACP)         : %s%s%s", statusColor, statusText, consts.Reset)
		case config.EthernetL4CheckerName:
			l4Print = fmt.Sprintf("L4(ARP)          : %s%s%s", statusColor, statusText, consts.Reset)
		case config.EthernetL5CheckerName:
			l5Print = fmt.Sprintf("L5(Routing)      : %s%s%s", statusColor, statusText, consts.Reset)
		}
	}

	ethInfo, ok := info.(*collector.EthernetInfo)
	if ok && len(ethInfo.BondInterfaces) > 0 {
		for _, bond := range ethInfo.BondInterfaces {
			fmt.Printf("Bond Interface: %s\n", bond)
			if sysfs, exists := ethInfo.SysfsBonding[bond]; exists {
				mode := sysfs["mode"]
				miimon := sysfs["miimon"]
				lacpRate := sysfs["lacp_rate"]
				slaves := strings.Join(ethInfo.BondSlaves[bond], ", ")

				fmt.Printf("Bond Mode: %-25s ", mode)
				fmt.Printf("MII Monitor: %-25s\n", miimon)
				fmt.Printf("LACP Rate: %-25s ", lacpRate)
				fmt.Printf("Slaves     : %-25s\n", slaves)
			}

			// Try parsing sysctl rp_filter
			if rpFilter, exists := ethInfo.RPFilter[bond]; exists {
				fmt.Printf("RP Filter: %-25s\n", rpFilter)
			}
			fmt.Println()
		}
	}

	fmt.Printf("%-35s%-35s\n", l1Print, l2Print)
	fmt.Printf("%-35s%-35s\n", l3Print, l4Print)
	fmt.Printf("%-35s\n", l5Print)

	if len(ethEvent) == 0 {
		fmt.Printf("\nErrors Events:\n\tNo Ethernet Events Detected\n")
	} else {
		fmt.Printf("\nErrors Events:\n")
		for _, v := range ethEvent {
			fmt.Printf("\t%s\n", v)
		}
	}

	fmt.Println()
	return checkAllPassed
}
