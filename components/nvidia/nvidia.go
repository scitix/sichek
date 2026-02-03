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
package nvidia

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/checker"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/components/nvidia/metrics"
	nvidiautils "github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	componentName string
	ctx           context.Context
	cancel        context.CancelFunc

	cfg         *config.NvidiaUserConfig
	cfgMutex    sync.RWMutex
	nvmlInst    nvml.Interface
	nvmlInstPtr *nvml.Interface // Shared pointer to NVML instance for collector and checkers
	collector   *collector.NvidiaCollector
	checkers    []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	xidPoller *XidEventPoller

	healthCheckMtx sync.Mutex
	serviceMtx     sync.RWMutex
	nvmlMtx        sync.RWMutex
	running        bool
	resultChannel  chan *common.Result

	metrics *metrics.NvidiaMetrics

	initError error // Track initialization errors with detailed information
}

var (
	nvidiaComponent     *component
	nvidiaComponentOnce sync.Once
)

func NewNvml(ctx context.Context) (nvml.Interface, error) {
	ctx_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	var nvmlInst nvml.Interface
	var initError error
	go func() {
		defer func() {
			if err := recover(); err != nil {
				initError = fmt.Errorf("panic occurred during NVML initialization: %v", err)
			}
			close(done)
		}()
		nvmlInst = nvml.New()
		if ret := nvmlInst.Init(); !errors.Is(ret, nvml.SUCCESS) {
			initError = fmt.Errorf("%v", nvml.ErrorString(ret))
		}
	}()
	select {
	case <-ctx_.Done():
		return nil, fmt.Errorf("NewNvml TIMEOUT")
	case <-done:
		if initError != nil {
			return nil, initError
		}
	}
	return nvmlInst, nil
}

/*
  - ReNewNvml reinitializes NVML instance.
    reinit nvmlInst becomes invalid during the program's runtime
    Key NVML Error Codes to Handle
    nvml.ERROR_UNINITIALIZED: NVML was not initialized or nvml.Shutdown was called.
    nvml.ERROR_DRIVER_NOT_LOADED: NVIDIA driver is not running or not installed.
    nvml.ERROR_NO_PERMISSION: Lack of access to the GPU.
    nvml.ERROR_GPU_IS_LOST: GPU is inaccessible, possibly due to a hardware failure.
    nvml.ERROR_UNKNOWN: An unexpected issue occurred
    If nvmlInst becomes invalid, reinitialize it.
*/
// Note: The caller must hold c.healthCheckMtx lock before calling this function
// to ensure thread-safe access to nvmlInst and xidPoller.
func ReNewNvml(c *component) error {

	// Stop the XidEventPoller before shutting down NVML to prevent SIGSEGV
	if c.xidPoller != nil {
		if err := c.xidPoller.Stop(); err != nil {
			logrus.WithField("component", "nvidia").Warningf("failed to stop xid poller before NVML reinit: %v", err)
		}
	}

	// Acquire write lock to prevent any NVML calls during Shutdown/New
	c.nvmlMtx.Lock()
	defer c.nvmlMtx.Unlock()
	if c.nvmlInst != nil {
		StopNvml(c.nvmlInst)
	}

	nvmlInst, ret := NewNvml(c.ctx)
	if ret != nil {
		c.nvmlInst = nil
		if c.nvmlInstPtr != nil {
			*c.nvmlInstPtr = nil
		}
	} else {
		c.nvmlInst = nvmlInst
		// Update the shared pointer - collector will automatically use the new instance
		if c.nvmlInstPtr != nil {
			*c.nvmlInstPtr = nvmlInst
		}
		// Recreate the XidEventPoller with the new NVML instance
		// Use RLock to check running status (nested lock: healthCheckMtx -> serviceMtx is safe)
		c.serviceMtx.RLock()
		isRunning := c.running
		c.serviceMtx.RUnlock()
		if isRunning {
			newXidPoller, err := NewXidEventPoller(c.ctx, c.cfg, nvmlInst, &c.nvmlMtx, c.resultChannel)
			if err != nil {
				logrus.WithField("component", "nvidia").Errorf("failed to recreate xid poller after NVML reinit: %v", err)
			} else {
				c.xidPoller = newXidPoller
				// Restart the poller in a goroutine if the component is still running
				go func() {
					defer func() {
						if err := recover(); err != nil {
							fmt.Printf("[xidPoller] panic err is %s\n", err)
						}
					}()
					err := c.xidPoller.Start()
					if err != nil {
						logrus.WithField("component", "nvidia").Errorf("start xid poller failed after reinit: %v", err)
					}
				}()
			}
		}
	}
	return ret
}

func StopNvml(nvmlInst nvml.Interface) {
	ret := nvmlInst.Shutdown()
	if !errors.Is(ret, nvml.SUCCESS) {
		logrus.WithField("component", "nvidia").Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
	}
}

func GetComponent() (common.Component, error) {
	if nvidiaComponent == nil {
		return nil, fmt.Errorf("nvidia component not initialized")
	}
	return nvidiaComponent, nil
}

func NewComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp common.Component, err error) {
	nvidiaComponentOnce.Do(func() {
		nvidiaComponent, err = newNvidia(cfgFile, specFile, ignoredCheckers)
	})
	// Ensure we never return nil component without error
	if nvidiaComponent == nil && err == nil {
		return nil, fmt.Errorf("nvidia component initialization failed: component is nil but no error was returned")
	}
	return nvidiaComponent, err
}

func newNvidia(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	component := &component{
		componentName:  consts.ComponentNameNvidia,
		ctx:            ctx,
		cancel:         cancel,
		cfgMutex:       sync.RWMutex{},
		healthCheckMtx: sync.Mutex{},
		serviceMtx:     sync.RWMutex{},
		nvmlMtx:        sync.RWMutex{},
		running:        false,
		resultChannel:  make(chan *common.Result),
	}

	nvidiaCfg := &config.NvidiaUserConfig{}
	err = common.LoadUserConfig(cfgFile, nvidiaCfg)
	if err != nil || nvidiaCfg.Nvidia == nil {
		logrus.WithField("component", "nvidia").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		component.initError = fmt.Errorf("get user config failed: %w", err)
		// Set a default config so Start() can still call HealthCheck to report initError
		defaultCfg := &config.NvidiaUserConfig{
			Nvidia: &config.NvidiaConfig{
				QueryInterval: common.Duration{Duration: 10 * time.Second},
				CacheSize:     5,
			},
		}
		component.cfg = defaultCfg
		return component, nil
	}
	if len(ignoredCheckers) > 0 {
		nvidiaCfg.Nvidia.IgnoredCheckers = ignoredCheckers
	}
	component.cfg = nvidiaCfg
	cacheSize := nvidiaCfg.Nvidia.CacheSize
	if cacheSize <= 0 {
		cacheSize = 1
	}
	component.cacheBuffer = make([]*common.Result, cacheSize)
	component.cacheInfo = make([]common.Info, cacheSize)
	component.currIndex = 0
	component.cacheSize = cacheSize

	nvmlInst, err := NewNvml(ctx)
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("NewNvidia create nvml failed: %v", err)
		component.initError = fmt.Errorf("NVML initialization failed: %w", err)
		return component, nil
	}

	// Create a shared pointer to NVML instance so all components can share it
	// When ReNewNvml updates this pointer, all components will automatically use the new instance
	component.nvmlInst = nvmlInst
	component.nvmlInstPtr = &nvmlInst

	nvidiaSpecCfg, err := config.LoadSpec(specFile)
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("LoadSpec failed: %v", err)
		component.initError = fmt.Errorf("spec loading failed: %w", err)
		return component, nil
	}
	if nvidiaSpecCfg == nil {
		logrus.WithField("component", "nvidia").Errorf("LoadSpec returned nil spec")
		component.initError = fmt.Errorf("NVIDIA spec is nil after loading from %s", specFile)
		return component, nil
	}

	// Pass the shared pointer to collector
	// Note: NVML calls in collector are protected by locks in nvidia.go where collector methods are called
	component.nvmlMtx.Lock()
	collectorPointer, err := collector.NewNvidiaCollector(ctx, component.nvmlInstPtr, nvidiaSpecCfg.GpuNums, nvidiaSpecCfg.Name)
	component.nvmlMtx.Unlock()
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("NewNvidiaCollector failed: %v", err)
		component.initError = fmt.Errorf("failed to create nvidia collector: %w", err)
		return component, nil
	}

	checkers, err := checker.NewCheckers(nvidiaCfg, nvidiaSpecCfg)
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("NewCheckers failed: %v", err)
		component.initError = fmt.Errorf("failed to create nvidia checkers: %w", err)
		return component, nil
	}

	xidPoller, err := NewXidEventPoller(ctx, nvidiaCfg, nvmlInst, &component.nvmlMtx, component.resultChannel)
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("NewXidEventPoller failed: %v", err)
		component.initError = fmt.Errorf("failed to create XID event poller: %w", err)
		return component, nil
	}

	freqController := common.GetFreqController()
	freqController.RegisterModule(consts.ComponentNameNvidia, nvidiaCfg)

	var nvidiaMetrics *metrics.NvidiaMetrics
	if nvidiaCfg.Nvidia.EnableMetrics {
		nvidiaMetrics = metrics.NewNvidiaMetrics()
	}

	// Complete component initialization
	component.collector = collectorPointer
	component.checkers = checkers
	component.xidPoller = xidPoller
	component.metrics = nvidiaMetrics

	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

// checkInitError checks for initialization errors and returns an error result if found.
func (c *component) checkInitError() (*common.Result, bool) {
	if c.initError == nil {
		return nil, false
	}

	logrus.WithField("component", "nvidia").Errorf("report initError: %v", c.initError)
	checkerResult := &common.CheckerResult{
		Name:        "InitError",
		Description: "Nvidia component initialization failed",
		Status:      consts.StatusAbnormal,
		Level:       consts.LevelCritical,
		Curr:        c.initError.Error(),
		ErrorName:   "InitError",
		Suggestion:  "Please check the initialization logs and ensure all dependencies are properly configured",
	}
	result := &common.Result{
		Item:     consts.ComponentNameNvidia,
		Status:   consts.StatusAbnormal,
		Checkers: []*common.CheckerResult{checkerResult},
		Time:     time.Now(),
	}
	return result, true
}

func (c *component) reportInitNVMLError(err error) *common.Result {
	checkerResult := &common.CheckerResult{
		Name:        "NVMLInitFailed",
		Description: fmt.Sprintf("failed to reinitialize NVML: %v", err),
		Status:      consts.StatusAbnormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "NVMLInitFailed",
		Suggestion:  fmt.Sprintf("Please check the %s status", c.componentName),
	}
	result := &common.Result{
		Item:     consts.ComponentNameNvidia,
		Status:   consts.StatusAbnormal,
		Checkers: []*common.CheckerResult{checkerResult},
		Time:     time.Now(),
	}
	return result
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	c.healthCheckMtx.Lock()
	defer c.healthCheckMtx.Unlock()

	// Check for initialization errors
	if result, hasError := c.checkInitError(); hasError {
		return result, nil
	}

	// Try to reinitialize NVML if instance is nil (from previous failed check)
	if c.nvmlInst == nil {
		err := ReNewNvml(c)
		if err != nil {
			return c.reportInitNVMLError(err), nil
		}
		logrus.WithField("component", "nvidia").Infof("reinitialized NVML successfully")
	}

	timer := common.NewTimer(fmt.Sprintf("%s-HealthCheck-Cost", c.componentName))
	// Protect all NVML calls in collector with RLock
	c.nvmlMtx.RLock()
	nvidiaInfo, err := c.collector.Collect(ctx)
	c.nvmlMtx.RUnlock()
	timer.Mark("Collect")

	if err != nil {
		// Check if the error indicates NVML instance is invalid
		if nvidiautils.CheckNvmlInvalidError(err) {
			// Mark NVML instance as invalid, will be reinitialized on next HealthCheck
			c.nvmlInst = nil
			if c.nvmlInstPtr != nil {
				*c.nvmlInstPtr = nil
			}
			logrus.WithField("component", "nvidia").Errorf("NVML instance is invalid, marking for reinitialization on next check: %v", err)

			return c.reportInitNVMLError(err), nil
		}

		// Other errors during collection, return error result
		logrus.WithField("component", "nvidia").Errorf("failed to collect nvidia info: %v", err)
		checkerResult := &common.CheckerResult{
			Name:        "CollectFailed",
			Description: fmt.Sprintf("failed to collect NVIDIA device information: %v", err),
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      "",
			ErrorName:   "CollectFailed",
			Suggestion:  fmt.Sprintf("Please check the %s status", c.componentName),
		}
		result := &common.Result{
			Item:     consts.ComponentNameNvidia,
			Status:   consts.StatusAbnormal,
			Checkers: []*common.CheckerResult{checkerResult},
			Time:     time.Now(),
		}
		return result, fmt.Errorf("failed to collect nvidia info: %w", err)
	}

	// Successfully collected info, continue with normal flow
	if c.cfg.Nvidia.EnableMetrics {
		c.metrics.ExportMetrics(nvidiaInfo)
	}
	result := common.Check(ctx, c.componentName, nvidiaInfo, c.checkers)
	timer.Mark("check")
	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = nvidiaInfo
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "nvidia").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "nvidia").Infof("Health Check PASSED")
	}
	timer.Mark("cache")

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

func (c *component) Start() <-chan *common.Result {
	c.serviceMtx.Lock()
	if c.running {
		c.serviceMtx.Unlock()
		return c.resultChannel
	}
	c.running = true
	c.serviceMtx.Unlock()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("[NvidiaStart] panic err is %s\n", err)
			}
		}()
		c.cfgMutex.RLock()
		// cfg is guaranteed to be set during initialization, so no nil check needed
		interval := c.cfg.GetQueryInterval()
		c.cfgMutex.RUnlock()
		logrus.WithField("component", "nvidia").Infof("Starting NVIDIA component with query_interval: %s", interval.Duration)
		ticker := time.NewTicker(interval.Duration)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// Check if need to update ticker
				c.cfgMutex.RLock()
				// cfg is guaranteed to be set during initialization, so no nil check needed
				newInterval := c.cfg.GetQueryInterval()
				c.cfgMutex.RUnlock()
				if newInterval != interval {
					logrus.WithField("component", "nvidia").Infof("Updating ticker interval from %s to %s", interval.Duration, newInterval.Duration)
					ticker.Stop()
					ticker = time.NewTicker(newInterval.Duration)
					interval = newInterval
				}
				result, err := common.RunHealthCheckWithTimeout(c.ctx, c.GetTimeout(), c.componentName, c.HealthCheck)
				if err != nil {
					fmt.Printf("%s HealthCheck failed: %v\n", c.componentName, err)
					continue
				}
				// Check if the error message contains "Timeout"
				if result != nil && len(result.Checkers) > 0 && strings.Contains(result.Checkers[0].Name, "HealthCheckTimeout") {
					// Handle the timeout error
					// ReNewNvml requires healthCheckMtx lock to be held
					c.healthCheckMtx.Lock()
					err := ReNewNvml(c)
					c.healthCheckMtx.Unlock()
					if err != nil {
						logrus.WithField("component", "nvidia").Errorf("failed to Reinitialize NVML after HealthCheck Timeout: %s", err.Error())
					} else {
						logrus.WithField("component", "nvidia").Warnf("Reinitialize NVML successfully after HealthCheck Timeout")
					}
				}
				c.checkXidPollerResult(result)
				c.serviceMtx.Lock()
				c.resultChannel <- result
				c.serviceMtx.Unlock()
			}
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("[xidPoller] panic err is %s\n", err)
			}
		}()
		if c.xidPoller == nil {
			return
		}
		err := c.xidPoller.Start()
		if err != nil {
			logrus.WithField("component", "nvidia").Errorf("start xid poller failed: %v", err)
		}
	}()

	c.serviceMtx.Lock()
	c.running = true
	c.serviceMtx.Unlock()
	return c.resultChannel
}

func (c *component) Stop() error {
	c.cancel()
	c.serviceMtx.Lock()
	close(c.resultChannel)
	c.running = false
	c.serviceMtx.Unlock()

	// Stop the XidEventPoller to properly clean up resources
	if c.xidPoller != nil {
		if err := c.xidPoller.Stop(); err != nil {
			logrus.WithField("component", "nvidia").Errorf("failed to stop xid poller: %v", err)
		}
	}

	return nil
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.NvidiaUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for nvidia")
	}
	c.cfg = configPointer
	c.cfgMutex.Unlock()
	return nil
}

func (c *component) Status() bool {
	c.serviceMtx.RLock()
	defer c.serviceMtx.RUnlock()

	return c.running
}

func (c *component) GetTimeout() time.Duration {
	c.cfgMutex.RLock()
	defer c.cfgMutex.RUnlock()
	return c.cfg.GetQueryInterval().Duration
}

func (c *component) checkXidPollerResult(result *common.Result) {
	xid := uint64(0)
	for _, res := range result.Checkers {
		if strings.Contains(res.Name, "xid") {
			parts := strings.Split(res.Name, "-")
			if len(parts) > 1 {
				xidStr := parts[1]
				var err error
				xid, err = strconv.ParseUint(xidStr, 10, 64)
				if err != nil {
					logrus.WithField("component", "nvidia").Errorf("checkXidPollerResult Error converting string to uint64: %v", err)
					return
				}
			} else {
				logrus.WithField("component", "nvidia").Errorf("checkXidPollerResult Unexpected Xid Err Name: %s\n", res.Name)
				return
			}
		}
	}

	for key, item := range config.CriticalXidEvent {
		if key == xid {
			continue
		}
		item.Status = consts.StatusNormal
		result.Checkers = append(result.Checkers, &item)
	}
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	nvidiaInfo, ok := info.(*collector.NvidiaInfo)
	if !ok {
		logrus.WithField("component", "cpu").Errorf("invalid data type, expected NvidiaInfo")
		return false
	}
	checkerResults := result.Checkers
	var (
		driverPrint        string
		iommuPrint         string
		persistencePrint   string
		cudaVersionPrint   string
		acsPrint           string
		nvlinkPrint        string
		pcieLinkPrint      string
		peermemPrint       string
		pstatePrint        string
		gpuStatusPrint     string
		fabricmanagerPrint string
		ibgdaPrint         string
		p2pPrint           string
	)
	systemEvent := make(map[string]string)
	gpuStatus := make(map[string]string)
	clockEvents := make(map[string]string)
	eccEvents := make(map[string]string)
	remmapedRowsEvents := make(map[string]string)
	// softErrorsEvents   := make(map[string]string)
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			checkAllPassed = false
		}
		switch result.Name {
		case config.PCIeACSCheckerName:
			if result.Status == consts.StatusNormal {
				acsPrint = fmt.Sprintf("PCIe ACS: %sDisabled%s", consts.Green, consts.Reset)
				if result.Curr != "Disabled" {
					systemEvent[config.PCIeACSCheckerName] = fmt.Sprintf("%s%s%s", consts.Yellow, result.Detail, consts.Reset)
				}
			} else {
				acsPrint = fmt.Sprintf("PCIe ACS: %sEnabled%s", consts.Red, consts.Reset)
				systemEvent[config.PCIeACSCheckerName] = fmt.Sprintf("%sNot All PCIe ACS Are Disabled%s", consts.Red, consts.Reset)
			}
		case config.IBGDACheckerName:
			if result.Status == consts.StatusNormal {
				ibgdaPrint = fmt.Sprintf("IBGDA: %sEnabled%s", consts.Green, consts.Reset)
			} else {
				ibgdaPrint = fmt.Sprintf("IBGDA: %sDisabled%s", consts.Red, consts.Reset)
				systemEvent[config.IBGDACheckerName] = fmt.Sprintf("%sIBGDA is not enabled correctly%s", consts.Red, consts.Reset)
				systemEvent[config.IBGDACheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.P2PCheckerName:
			if result.Status == consts.StatusNormal {
				if strings.Contains(result.Detail, "Disabled") {
					p2pPrint = fmt.Sprintf("P2P: %sNotSupported%s", consts.Yellow, consts.Reset)
				} else {
					p2pPrint = fmt.Sprintf("P2P: %sOK%s", consts.Green, consts.Reset)
				}
			} else {
				p2pPrint = fmt.Sprintf("P2P: %sError%s", consts.Red, consts.Reset)
				systemEvent[config.P2PCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.IOMMUCheckerName:
			if result.Status == consts.StatusNormal {
				iommuPrint = fmt.Sprintf("IOMMU: %sOFF%s", consts.Green, consts.Reset)
			} else {
				iommuPrint = fmt.Sprintf("IOMMU: %sON%s", consts.Red, consts.Reset)
				systemEvent[config.IOMMUCheckerName] = fmt.Sprintf("%sIOMMU is ON%s", consts.Red, consts.Reset)
			}
		case config.NVFabricManagerCheckerName:
			if result.Status == consts.StatusNormal {
				fabricmanagerPrint = fmt.Sprintf("FabricManager: %s%s%s", consts.Green, result.Curr, consts.Reset)
				if result.Curr == "NotActive" {
					gpuStatus[config.NVFabricManagerCheckerName] = fmt.Sprintf("%s%s%s", consts.Yellow, result.Detail, consts.Reset)
				}
			} else {
				fabricmanagerPrint = fmt.Sprintf("FabricManager: %sNot Active%s", consts.Red, consts.Reset)
				gpuStatus[config.NVFabricManagerCheckerName] = fmt.Sprintf("%sNvidia FabricManager is not active%s", consts.Red, consts.Reset)
			}
		case config.NvPeerMemCheckerName:
			if result.Status == consts.StatusNormal {
				peermemPrint = fmt.Sprintf("nvidia_peermem: %sLoaded%s", consts.Green, consts.Reset)
				if result.Curr != "Loaded" {
					gpuStatus[config.NvPeerMemCheckerName] = fmt.Sprintf("%s%s%s", consts.Yellow, result.Detail, consts.Reset)
				}
			} else {
				peermemPrint = fmt.Sprintf("nvidia_peermem: %sNotLoaded%s", consts.Red, consts.Reset)
				gpuStatus[config.NvPeerMemCheckerName] = fmt.Sprintf("%snvidia_peermem module: NotLoaded%s", consts.Red, consts.Reset)
			}
		case config.PCIeCheckerName:
			if result.Status == consts.StatusNormal {
				pcieLinkPrint = fmt.Sprintf("PCIeLink: %sOK%s", consts.Green, consts.Reset)
			} else {
				gpuStatus[config.PCIeCheckerName] = fmt.Sprintf("%sPCIe degradation detected:\n%s%s%s", consts.Red, consts.Yellow, result.Detail, consts.Reset)
			}
		case config.HardwareCheckerName:
			if result.Status == consts.StatusNormal {
				gpuStatusPrint = fmt.Sprintf("%s%d%s GPUs detected, %s%d%s GPUs used",
					consts.Green, nvidiaInfo.DeviceCount, consts.Reset, consts.Green, nvidiaInfo.DeviceUsedCount, consts.Reset)
			} else {
				gpuStatusPrint = fmt.Sprintf("%s%d GPUs detected, %d GPUs lost, %d GPUs used%s",
					consts.Red, nvidiaInfo.DeviceCount, len(strings.Split(result.Device, ",")), nvidiaInfo.DeviceUsedCount, consts.Reset)
				gpuStatus[config.HardwareCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.SoftwareCheckerName:
			if result.Status == consts.StatusNormal {
				driverPrint = fmt.Sprintf("Driver Version: %s%s%s", consts.Green, nvidiaInfo.SoftwareInfo.DriverVersion, consts.Reset)
				cudaVersionPrint = fmt.Sprintf("CUDA Version: %s%s%s", consts.Green, nvidiaInfo.SoftwareInfo.CUDAVersion, consts.Reset)
			} else {
				driverPrint = fmt.Sprintf("Driver Version: %s%s%s", consts.Red, nvidiaInfo.SoftwareInfo.DriverVersion, consts.Reset)
				cudaVersionPrint = fmt.Sprintf("CUDA Version: %s%s%s", consts.Red, nvidiaInfo.SoftwareInfo.CUDAVersion, consts.Reset)
				gpuStatus[config.SoftwareCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.GpuPersistencedCheckerName:
			if result.Status == consts.StatusNormal {
				persistencePrint = fmt.Sprintf("Persistence Mode: %s%s%s", consts.Green, result.Curr, consts.Reset)
				if result.Curr != "Enabled" {
					gpuStatus[config.GpuPersistencedCheckerName] = fmt.Sprintf("%s%s%s", consts.Yellow, result.Detail, consts.Reset)
				}
			} else {
				persistencePrint = fmt.Sprintf("Persistence Mode: %s%s%s", consts.Red, result.Curr, consts.Reset)
				gpuStatus[config.GpuPersistencedCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.GpuPStateCheckerName:
			if result.Status == consts.StatusNormal {
				pstatePrint = fmt.Sprintf("PState: %s%s%s", consts.Green, result.Curr, consts.Reset)
			} else {
				pstatePrint = fmt.Sprintf("PState: %s%s%s", consts.Red, result.Curr, consts.Reset)
				gpuStatus[config.GpuPStateCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.NvlinkCheckerName:
			if result.Status == consts.StatusNormal {
				nvlinkPrint = fmt.Sprintf("NVLink: %s%s%s", consts.Green, result.Curr, consts.Reset)
			} else {
				nvlinkPrint = fmt.Sprintf("NVLink: %s%s%s", consts.Green, result.Curr, consts.Reset)
				gpuStatus[config.NvlinkCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.AppClocksCheckerName:
			if result.Status == consts.StatusNormal {
				// gpuStatus[config.AppClocksCheckerName] = fmt.Sprintf("%sGPU application clocks: Set to maximum%s", consts.Green, consts.Reset)
			} else {
				gpuStatus[config.AppClocksCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.ClockEventsCheckerName:
			if result.Status == consts.StatusNormal {
				clockEvents["Thermal"] = fmt.Sprintf("%sNo HW Thermal Slowdown Found%s", consts.Green, consts.Reset)
				clockEvents["PowerBrake"] = fmt.Sprintf("%sNo HW Power Brake Slowdown Found%s", consts.Green, consts.Reset)
			} else {
				clockEvents["Thermal"] = fmt.Sprintf("%sHW Thermal Slowdown Found%s", consts.Red, consts.Reset)
				clockEvents["PowerBrake"] = fmt.Sprintf("%sHW Power Brake Slowdown Found%s", consts.Red, consts.Reset)
			}
		case config.SRAMAggUncorrectableCheckerName:
			if result.Status == consts.StatusNormal {
				eccEvents[config.SRAMAggUncorrectableCheckerName] = fmt.Sprintf("%sNo SRAM Agg Uncorrectable Found%s", consts.Green, consts.Reset)
			} else {
				eccEvents[config.SRAMAggUncorrectableCheckerName] = fmt.Sprintf("%sSRAM Agg Uncorrectable Found\n%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.SRAMHighcorrectableCheckerName:
			if result.Status == consts.StatusNormal {
				eccEvents[config.SRAMHighcorrectableCheckerName] = fmt.Sprintf("%sNo SRAM High Correctable Found%s", consts.Green, consts.Reset)
			} else {
				eccEvents[config.SRAMHighcorrectableCheckerName] = fmt.Sprintf("%sSRAM High Correctable Found\n%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.SRAMVolatileUncorrectableCheckerName:
			if result.Status == consts.StatusNormal {
				eccEvents[config.SRAMVolatileUncorrectableCheckerName] = fmt.Sprintf("%sNo SRAM Volatile Uncorrectable Found%s", consts.Green, consts.Reset)
			} else {
				eccEvents[config.SRAMVolatileUncorrectableCheckerName] = fmt.Sprintf("%sSRAM Volatile Uncorrectable Found\n%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.RemmapedRowsFailureCheckerName:
			if result.Status == consts.StatusNormal {
				remmapedRowsEvents[config.RemmapedRowsFailureCheckerName] = fmt.Sprintf("%sNo Remmaped Rows Failure Found%s", consts.Green, consts.Reset)
			} else {
				remmapedRowsEvents[config.RemmapedRowsFailureCheckerName] = fmt.Sprintf("%sRemmaped Rows Failure Found\n%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.RemmapedRowsUncorrectableCheckerName:
			if result.Status == consts.StatusNormal {
				remmapedRowsEvents[config.RemmapedRowsUncorrectableCheckerName] = fmt.Sprintf("%sNo Remmaped Rows Uncorrectable Found%s", consts.Green, consts.Reset)
			} else {
				remmapedRowsEvents[config.RemmapedRowsUncorrectableCheckerName] = fmt.Sprintf("%sRemmaped Rows Uncorrectable Found\n%s%s", consts.Red, result.Detail, consts.Reset)
			}
		case config.RemmapedRowsPendingCheckerName:
			if result.Status == consts.StatusNormal {
				remmapedRowsEvents[config.RemmapedRowsPendingCheckerName] = fmt.Sprintf("%sNo Remmaped Rows Pending Found%s", consts.Green, consts.Reset)
			} else {
				remmapedRowsEvents[config.RemmapedRowsPendingCheckerName] = fmt.Sprintf("%sRemmaped Rows Pending Found\n%s%s", consts.Red, result.Detail, consts.Reset)
			}
		}
	}
	if summaryPrint {
		utils.PrintTitle("NVIDIA GPUs", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 40
		if err == nil {
			printInterval = termWidth / 3
		}

		gpuNumPrint := "GPU NUMs:"
		if len(nvidiaInfo.DevicesInfo) > 0 {
			fmt.Printf("%s\n", nvidiaInfo.DevicesInfo[0].Name)
		} else {
			fmt.Printf("No GPU devices available\n")
		}
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, driverPrint, printInterval, iommuPrint, printInterval, persistencePrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, cudaVersionPrint, printInterval, acsPrint, printInterval, pstatePrint)
		fmt.Printf("%-*s%-*s\n", printInterval, p2pPrint, printInterval, ibgdaPrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval-consts.PadLen, gpuNumPrint, printInterval, peermemPrint, printInterval, nvlinkPrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval+consts.PadLen, gpuStatusPrint, printInterval, fabricmanagerPrint, printInterval, pcieLinkPrint)
		fmt.Println()
	}
	if len(systemEvent) > 0 {
		fmt.Println("System Settings and Status:")
		for _, v := range systemEvent {
			fmt.Printf("\t%s\n", v)
		}
	}
	if len(gpuStatus) > 0 {
		fmt.Println("NVIDIA GPU:")
		for _, v := range gpuStatus {
			fmt.Printf("\t%s\n", v)
		}
	}
	fmt.Println("Clock Events:")
	for _, v := range clockEvents {
		fmt.Printf("\t%s\n", v)
	}
	fmt.Println("Memory ECC:")
	for _, v := range eccEvents {
		fmt.Printf("\t%s\n", v)
	}
	fmt.Println("Remapped Rows:")
	for _, v := range remmapedRowsEvents {
		fmt.Printf("\t%s\n", v)
	}
	return checkAllPassed
}
