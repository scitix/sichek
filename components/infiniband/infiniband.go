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
package infiniband

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/checker"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/components/infiniband/metrics"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

var (
	infinibandComponent     *component
	infinibandComponentOnce sync.Once
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	spec          *config.InfinibandSpecItem
	info          common.Info
	componentName string
	cfg           *config.InfinibandUserConfig
	cfgMutex      sync.RWMutex

	// collector common.Collector
	checkers []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
	metrics *metrics.IBMetrics
}

func NewInfinibandComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp common.Component, err error) {
	infinibandComponentOnce.Do(func() {
		infinibandComponent, err = newInfinibandComponent(cfgFile, specFile, ignoredCheckers)
		if err != nil {
			panic(err)
		}
	})
	return infinibandComponent, nil
}

func newInfinibandComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg := &config.InfinibandUserConfig{}
	err = cfg.LoadUserConfigFromYaml(cfgFile)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("NewComponent load user config failed: %v", err)
		return nil, err
	}
	if len(ignoredCheckers) > 0 {
		cfg.Infiniband.IgnoredCheckers = ignoredCheckers
	}
	ibSpecs := &config.InfinibandSpecConfig{}
	err = ibSpecs.LoadSpecConfigFromYaml(specFile)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("NewComponent load spec config failed: %v", err)
		return nil, err
	}
	ibSpec, err := ibSpecs.GetClusterInfinibandSpec()
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("NewComponent load spec config failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(cfg, ibSpec)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("NewCheckers failed: %v", err)
		return nil, err
	}

	var info collector.InfinibandInfo
	var infinibandMetrics *metrics.IBMetrics
	if cfg.Infiniband.EnableMetrics {
		infinibandMetrics = metrics.NewInfinibandMetrics()
	}
	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		spec:          ibSpec,
		componentName: consts.ComponentNameInfiniband,
		info:          info.GetIBInfo(ctx),
		checkers:      checkers,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cfg.Infiniband.CacheSize),
		cacheInfo:     make([]common.Info, cfg.Infiniband.CacheSize),
		currIndex:     0,
		cacheSize:     cfg.Infiniband.CacheSize,
		metrics:       infinibandMetrics,
	}
	// step4: start the service
	component.service = common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)

	// step5: return the component
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	InfinibandInfo, ok := c.info.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("expected c.info to be of type *collector.InfinibandInfo, got %T", c.info)
	}
	if c.cfg.Infiniband.EnableMetrics {
		c.metrics.ExportMetrics(InfinibandInfo)
	}
	result := common.Check(ctx, c.componentName, InfinibandInfo, c.checkers)
	info, err := InfinibandInfo.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert infiniband info to JSON: %w", err)
	}
	result.RawData = info

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = InfinibandInfo
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "Infiniband").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "Infiniband").Infof("Health Check PASSED")
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

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

// 更新组件的配置信息，同时更新service
func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	config, ok := cfg.(*config.InfinibandUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for infiniband")
	}
	c.cfg = config
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

// Start方法用于systemD的启动，周期性地执行HealthCheck函数获取数据，并将结果发送到resultChannel
func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

// 返回组件的运行情况
func (c *component) Status() bool {
	return c.service.Status()
}

// 用于systemD的停止
func (c *component) Stop() error {
	return c.service.Stop()

}

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval() * time.Second
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true

	ibInfo, ok := info.(*collector.InfinibandInfo)
	if !ok {
		logrus.WithField("component", "infiniband").Errorf("invalid data type, expected InfinibandInfo")
		return false
	}

	checkerResults := result.Checkers
	ibControllersPrintColor := consts.Green
	// PerformancePrint := "Performance: "

	var (
		ibKmodPrint      string
		ofedVersionPrint string
		fwVersionPrint   string
		ibPortSpeedPrint string
		phyStatPrint     string
		ibStatePrint     string
		pcieLinkPrint    string
		// throughPrint        string
		// latencyPrint     string
	)
	pcieGen := ""
	pcieWidth := ""

	infinibandEvents := make(map[string]string)
	ofedVersionPrint = fmt.Sprintf("OFED Version: %s%s%s", consts.Green, ibInfo.IBSoftWareInfo.OFEDVer, consts.Reset)

	logrus.Infof("checkerResults: %v", common.ToString(checkerResults))

	for _, result := range checkerResults {
		statusColor := consts.Green
		if result.Status != consts.StatusNormal {
			statusColor = consts.Red
			infinibandEvents[result.Name] = fmt.Sprintf("%s%s%s", statusColor, result.Detail, consts.Reset)
			checkAllPassed = false
		}

		switch result.Name {
		case config.ChekIBOFED:
			ofedVersionPrint = fmt.Sprintf("OFED Version: %s%s%s", statusColor, result.Curr, consts.Reset)
		case config.CheckIBKmod:
			ibKmodPrint = fmt.Sprintf("Infiniband Kmod: %s%s%s", statusColor, "Loaded", consts.Reset)
			if result.Status != consts.StatusNormal {
				ibKmodPrint = fmt.Sprintf("Infiniband Kmod: %s%s%s", statusColor, "Not Loaded Correctly", consts.Reset)
			}
		case config.ChekIBFW:
			fwVersion := extractAndDeduplicate(result.Curr)
			fwVersionPrint = fmt.Sprintf("FW Version: %s%s%s", statusColor, fwVersion, consts.Reset)
		case config.ChekIBPortSpeed:
			portSpeed := extractAndDeduplicate(result.Curr)
			ibPortSpeedPrint = fmt.Sprintf("IB Port Speed: %s%s%s", statusColor, portSpeed, consts.Reset)
		case config.ChekIBPhyState:
			phyState := "LinkUp"
			if result.Status != consts.StatusNormal {
				phyState = "Not All LinkUp"
			}
			phyStatPrint = fmt.Sprintf("Phy State: %s%s%s", statusColor, phyState, consts.Reset)
		case config.ChekIBState:
			ibState := "Active"
			if result.Status != consts.StatusNormal {
				ibState = "Not All Active"
			}
			ibStatePrint = fmt.Sprintf("IB State: %s%s%s", statusColor, ibState, consts.Reset)
		case config.CheckPCIESpeed:
			pcieGen = fmt.Sprintf("%s%s%s", statusColor, extractAndDeduplicate(result.Curr), consts.Reset)
		case config.CheckPCIEWidth:
			pcieWidth = fmt.Sprintf("%s%s%s", statusColor, extractAndDeduplicate(result.Curr), consts.Reset)
		case config.CheckIBDevs:
			ibControllersPrintColor = statusColor
		}
	}
	if pcieGen != "" && pcieWidth != "" {
		pcieLinkPrint = fmt.Sprintf("PCIe Link: %s%s (x%s)%s", consts.Green, pcieGen, pcieWidth, consts.Reset)
	} else {
		pcieLinkPrint = fmt.Sprintf("PCIe Link: %sError Detected%s", consts.Red, consts.Reset)
	}

	ibControllersPrint := fmt.Sprintf("Host Channel Adaptor: %s", ibControllersPrintColor)
	for _, hwInfo := range ibInfo.IBHardWareInfo {
		ibControllersPrint += fmt.Sprintf("%s(%s), ", hwInfo.IBDev, hwInfo.NetDev)
	}

	ibControllersPrint = strings.TrimSuffix(ibControllersPrint, ", ")
	ibControllersPrint += consts.Reset

	if summaryPrint {
		utils.PrintTitle("infiniband", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 60
		if err == nil {
			printInterval = termWidth / 3
		}
		if printInterval < len(ofedVersionPrint) {
			printInterval = len(ofedVersionPrint) + 2
		}
		fmt.Printf("%-*s\n", printInterval, ibControllersPrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, ibKmodPrint, printInterval, phyStatPrint, printInterval, "")          //, PerformancePrint)
		fmt.Printf("%-*s%-*s\t%-*s\n", printInterval, ofedVersionPrint, printInterval, ibStatePrint, printInterval, "")   //, "Throughput: TBD")
		fmt.Printf("%-*s%-*s\t%-*s\n", printInterval, fwVersionPrint, printInterval, ibPortSpeedPrint, printInterval, "") //, "Latency: TBD")
		fmt.Printf("%-*s%-*s\n", printInterval, consts.Green+""+consts.Reset, printInterval, pcieLinkPrint)
	}

	fmt.Println("Errors Events:")

	if len(infinibandEvents) == 0 {
		fmt.Printf("\t%sNo Infiniband Events Detected%s\n", consts.Green, consts.Reset)
	} else {
		for _, event := range infinibandEvents {
			fmt.Printf("\t%s\n", event)
		}
	}
	return checkAllPassed
}

func extractAndDeduplicate(curr string) string {
	// Split the string by ';'
	values := strings.Split(curr, ",")

	// Use a map to store unique values
	uniqueValues := make(map[string]struct{})
	for _, value := range values {
		if value != "" { // Ignore empty strings
			uniqueValues[value] = struct{}{}
		}
	}

	// Collect keys from the map into a slice
	result := make([]string, 0, len(uniqueValues))
	for key := range uniqueValues {
		result = append(result, key)
	}

	// Join the unique values back into a single string
	return strings.Join(result, ",")
}
