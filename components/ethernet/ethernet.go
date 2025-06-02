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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/checker"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/components/ethernet/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	ethernetComponent     *component
	ethernetComponentOnce sync.Once
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	spec          *config.EthernetSpecConfig
	info          common.Info
	componentName string
	cfg           *config.EthernetUserConfig
	cfgMutex      sync.RWMutex

	collector common.Collector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
	metrics *metrics.EthernetMetrics
}

func NewEthernetComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	ethernetComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component ethernet: %v", r)
			}
		}()
		ethernetComponent, err = newEthernetComponent(cfgFile, specFile)
	})
	return ethernetComponent, err
}

func newEthernetComponent(cfgFile string, specFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg := &config.EthernetUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Ethernet == nil {
		logrus.WithField("component", "ethernet").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewEthernetComponent get user config failed")
	}
	specCfg := &config.EthernetSpecConfig{}
	err = specCfg.LoadSpecConfigFromYaml(specFile)
	if err != nil {
		logrus.WithField("component", "ethernet").Errorf("NewComponent load spec config failed: %v", err)
		return nil, err
	}
	ethernetSpec := specCfg.EthernetSpec
	checkers, err := checker.NewCheckers(cfg, ethernetSpec)

	collector, err := collector.NewEthCollector()
	if err != nil {
		logrus.WithField("component", "ethernet").WithError(err).Error("failed to create ethernet collector")
	}

	var ethernetMetrics *metrics.EthernetMetrics
	if cfg.Ethernet.EnableMetrics {
		ethernetMetrics = metrics.NewEthernetMetrics()
	}
	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		spec:          specCfg,
		componentName: consts.ComponentNameEthernet,
		checkers:      checkers,
		cfg:           cfg,
		collector:     collector,
		cacheBuffer:   make([]*common.Result, cfg.Ethernet.CacheSize),
		cacheInfo:     make([]common.Info, cfg.Ethernet.CacheSize),
		currIndex:     0,
		cacheSize:     cfg.Ethernet.CacheSize,
		metrics:       ethernetMetrics,
	}
	component.service = common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "ethernet").Errorf("failed to collect ethernet info: %v", err)
		return nil, err
	}

	ethernetInfo, ok := info.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("expected c.info to be of type *collector.EthernetInfo, got %T", info)
	}
	if c.cfg.Ethernet.EnableMetrics {
		c.metrics.ExportMetrics(ethernetInfo)
	}
	result := common.Check(ctx, c.Name(), ethernetInfo, c.checkers)

	// info, err = ethernetInfo.JSON()
	// if err != nil {
	// 	return nil, err
	// }
	// result.RawData = info

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = ethernetInfo
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
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

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Status() bool {
	return c.service.Status()
}

func (c *component) Stop() error {
	return c.service.Stop()

}

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval().Duration
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	ethInfo, ok := info.(*collector.EthernetInfo)
	if !ok {
		logrus.WithField("component", "ethernet").Errorf("invalid data type, expected EthernetInfo")
		return false
	}

	ethControllersPrint := "Ethernet Nic: "
	var phyStatPrint string

	ethernetEvents := make(map[string]string)
	for _, ethDev := range ethInfo.EthDevs {
		ethControllersPrint += fmt.Sprintf("%s, ", ethDev)
	}
	ethControllersPrint = ethControllersPrint[:len(ethControllersPrint)-2]

	checkerResults := result.Checkers
	for _, result := range checkerResults {
		switch result.Name {
		case config.ChekEthPhyState:
			if result.Status == consts.StatusNormal {
				phyStatPrint = fmt.Sprintf("Phy State: %sLinkUp%s", consts.Green, consts.Reset)
			} else {
				phyStatPrint = fmt.Sprintf("Phy State: %sLinkDown%s", consts.Red, consts.Reset)
				ethernetEvents["phy_state"] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
				checkAllPassed = false
			}
		}
	}

	if summaryPrint {
		utils.PrintTitle("Ethernet", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 40
		if err == nil {
			printInterval = termWidth / 3
		}

		fmt.Printf("%-*s%-*s\n", printInterval, ethControllersPrint, printInterval, phyStatPrint)
		fmt.Println()
	}
	fmt.Println("Errors Events:")
	if len(ethernetEvents) == 0 {
		fmt.Println("\tNo ethernet Events Detected")
		return checkAllPassed
	}
	fmt.Printf("%16s : %-10s\n", "checkItems", "checkDetail")
	for item, v := range ethernetEvents {
		fmt.Printf("%16s : %-10s\n", item, v)
	}
	return checkAllPassed
}
