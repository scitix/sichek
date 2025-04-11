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
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/checker"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	componentName string
	ctx           context.Context
	cancel        context.CancelFunc

	cfg      *config.NvidiaUserConfig
	cfgMutex sync.RWMutex // 用于更新时的锁

	nvmlInst  nvml.Interface
	collector *collector.NvidiaCollector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	xidPoller *XidEventPoller

	serviceMtx    sync.RWMutex
	running       bool
	resultChannel chan *common.Result
}

var (
	nvidiaComponent     *component
	nvidiaComponentOnce sync.Once
)

func NewNvml(ctx context.Context) (nvml.Interface, error) {
	ctx_, cancel := context.WithTimeout(ctx, 2*time.Second)
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
  - TODO: reinit nvmlInst becomes invalid during the program's runtime
    Key NVML Error Codes to Handle
    nvml.ERROR_UNINITIALIZED: NVML was not initialized or nvml.Shutdown was called.
    nvml.ERROR_DRIVER_NOT_LOADED: NVIDIA driver is not running or not installed.
    nvml.ERROR_NO_PERMISSION: Lack of access to the GPU.
    nvml.ERROR_GPU_IS_LOST: GPU is inaccessible, possibly due to a hardware failure.
    nvml.ERROR_UNKNOWN: An unexpected issue occurred
    If nvmlInst becomes invalid, reinitialize it.
*/
func ReNewNvml(c *component) error {
	StopNvml(c.nvmlInst)

	nvmlInst, ret := NewNvml(c.ctx)
	if ret != nil {
		c.nvmlInst = nil
	} else {
		c.nvmlInst = nvmlInst
	}
	return ret
}

func StopNvml(nvmlInst nvml.Interface) {
	ret := nvmlInst.Shutdown()
	if !errors.Is(ret, nvml.SUCCESS) {
		logrus.WithField("component", "NVIDIA").Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
	}
}

func GetComponent() common.Component {
	if nvidiaComponent == nil {
		panic("nvidia component not initialized")
	}
	return nvidiaComponent
}

func NewComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp common.Component, err error) {
	nvidiaComponentOnce.Do(func() {
		nvidiaComponent, err = newNvidia(cfgFile, specFile, ignoredCheckers)
	})
	return nvidiaComponent, err
}

func newNvidia(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	nvmlInst, err := NewNvml(ctx)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewNvidia create nvml failed: %v", err)
		return nil, err
	}
	nvidiaCfg := &config.NvidiaUserConfig{}
	err = nvidiaCfg.LoadUserConfigFromYaml(cfgFile)
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("NewComponent load user config failed: %v", err)
		return nil, err
	}
	if len(ignoredCheckers) > 0 {
		nvidiaCfg.Nvidia.IgnoredCheckers = ignoredCheckers
	}
	nvidiaSpecCfgs := &config.NvidiaSpecConfig{}
	nvidiaSpecCfg := nvidiaSpecCfgs.GetSpec(specFile)
	if nvidiaSpecCfg == nil {
		return nil, fmt.Errorf("get nvidia spec failed")
	}
	collectorPointer, err := collector.NewNvidiaCollector(ctx, nvmlInst, nvidiaSpecCfg.GpuNums)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewNvidiaCollector failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(nvidiaCfg, nvidiaSpecCfg, nvmlInst)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewCheckers failed: %v", err)
		return nil, err
	}

	resultChannel := make(chan *common.Result)
	xidPoller, err := NewXidEventPoller(ctx, nvidiaCfg, nvmlInst, resultChannel)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewXidEventPoller failed: %v", err)
		return nil, err
	}

	freqController := common.GetFreqController()
	freqController.RegisterModule(consts.ComponentNameNvidia, nvidiaCfg)
	component := &component{
		componentName: consts.ComponentNameNvidia,
		ctx:           ctx,
		cancel:        cancel,
		cfg:           nvidiaCfg,
		cfgMutex:      sync.RWMutex{},
		nvmlInst:      nvmlInst,
		collector:     collectorPointer,
		checkers:      checkers,
		cacheMtx:      sync.RWMutex{},
		cacheBuffer:   make([]*common.Result, nvidiaCfg.Nvidia.CacheSize),
		cacheInfo:     make([]common.Info, nvidiaCfg.Nvidia.CacheSize),
		currIndex:     0,
		cacheSize:     nvidiaCfg.Nvidia.CacheSize,
		xidPoller:     xidPoller,
		running:       false,
		resultChannel: resultChannel,
	}
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	if c.nvmlInst == nil {
		err := ReNewNvml(c)
		// Check again after reinitialization
		if c.nvmlInst == nil {
			logrus.WithField("component", "NVIDIA").Errorf("failed to Reinitialize NVML: %s", err.Error())
			checkerResult := &common.CheckerResult{
				Name:        "NVMLInitFailed",
				Description: fmt.Sprintf("failed to Reinitialize NVML: %s", err.Error()),
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
			return result, fmt.Errorf("failed to reinitialize NVML: %s", err.Error())
		}
		logrus.WithField("component", "NVIDIA").Errorf("Reinitialize NVML successfully")
	}
	timer := common.NewTimer(fmt.Sprintf("%s-HealthCheck-Cost", c.componentName))
	nvidiaInfo, err := c.collector.Collect(ctx)
	timer.Mark("Collect")
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("failed to collect nvidia info: %v", err)
		return nil, err
	}
	result := common.Check(ctx, c.componentName, nvidiaInfo, c.checkers)
	timer.Mark("check")
	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = nvidiaInfo
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "NVIDIA").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "NVIDIA").Infof("Health Check PASSED")
	}
	timer.Mark("cache")

	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
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
		interval := c.cfg.GetQueryInterval()
		logrus.WithField("component", "NVIDIA").Infof("Starting NVIDIA component with interval: %v", interval*time.Second)
		ticker := time.NewTicker(interval * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// Check if need to update ticker
				newInterval := c.cfg.GetQueryInterval()
				if newInterval != interval {
					logrus.WithField("component", "NVIDIA").Infof("Updating ticker interval from %v to %v", interval*time.Second, newInterval*time.Second)
					ticker.Stop()
					ticker = time.NewTicker(newInterval * time.Second)
					interval = newInterval
				}
				c.serviceMtx.Lock()
				result, err := common.RunHealthCheckWithTimeout(c.ctx, c.GetTimeout(), c.componentName, c.HealthCheck)
				c.serviceMtx.Unlock()
				if err != nil {
					fmt.Printf("%s HealthCheck failed: %v\n", c.componentName, err)
					continue
				}
				// Check if the error message contains "Timeout"
				if strings.Contains(result.Checkers[0].Name, "HealthCheckTimeout") {
					// Handle the timeout error
					logrus.WithField("component", "NVIDIA").Errorf("Health Check TIMEOUT")
					err := ReNewNvml(c)
					if err != nil {
						logrus.WithField("component", "NVIDIA").Errorf("failed to Reinitialize NVML after HealthCheck Timeout: %s", err.Error())
					} else {
						logrus.WithField("component", "NVIDIA").Warnf("Reinitialize NVML successfully after HealthCheck Timeout")
					}
				}

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
		err := c.xidPoller.Start()
		if err != nil {
			logrus.WithField("component", "NVIDIA").Errorf("start xid poller failed: %v", err)
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
	return c.cfg.GetQueryInterval() * time.Second
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
				if result.Curr != "Active" {
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
					consts.Green, nvidiaInfo.DeviceCount, consts.Reset, consts.Green, len(nvidiaInfo.DeviceToPodMap), consts.Reset)
			} else {
				gpuStatusPrint = fmt.Sprintf("%s%d%s GPUs detected, %s%d%s GPUs used",
					consts.Red, nvidiaInfo.DeviceCount, consts.Reset, consts.Green, len(nvidiaInfo.DeviceToPodMap), consts.Reset)
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
		case config.GpuPersistenceCheckerName:
			if result.Status == consts.StatusNormal {
				persistencePrint = fmt.Sprintf("Persistence Mode: %s%s%s", consts.Green, result.Curr, consts.Reset)
				if result.Curr != "Enabled" {
					gpuStatus[config.GpuPersistenceCheckerName] = fmt.Sprintf("%s%s%s", consts.Yellow, result.Detail, consts.Reset)
				}
			} else {
				persistencePrint = fmt.Sprintf("Persistence Mode: %s%s%s", consts.Red, result.Curr, consts.Reset)
				gpuStatus[config.GpuPersistenceCheckerName] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
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
		fmt.Printf("%s\n", nvidiaInfo.DevicesInfo[0].Name)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, driverPrint, printInterval, iommuPrint, printInterval, persistencePrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, cudaVersionPrint, printInterval, acsPrint, printInterval, pstatePrint)
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
