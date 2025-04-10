package component

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/components/dmesg"
	"github.com/scitix/sichek/components/gpfs"
	"github.com/scitix/sichek/components/hang"
	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/components/nccl"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

var (
	ComponentStatuses = make(map[string]bool) // Tracks pass/fail status for each component
	StatusMutex       sync.Mutex              // Ensures thread-safe updates
)

type CheckResults struct {
	component common.Component
	result    *common.Result
	info      common.Info
}

func newComponent(componentName string, cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	switch componentName {
	case consts.ComponentNameGpfs:
		return gpfs.NewGpfsComponent(cfgFile)
	case consts.ComponentNameCPU:
		return cpu.NewComponent(cfgFile)
	case consts.ComponentNameInfiniband:
		return infiniband.NewInfinibandComponent(cfgFile, specFile, ignoredCheckers)
	case consts.ComponentNameDmesg:
		return dmesg.NewComponent(cfgFile)
	case consts.ComponentNameHang:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing Hang HealthCheck")
		}
		_, err := nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
		if err != nil {
			return nil, fmt.Errorf("failed to Get Nvidia component, Bypassing HealthCheck")
		}
		return hang.NewComponent(cfgFile)
	case consts.ComponentNameNvidia:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing Hang HealthCheck")
		}
		return nvidia.NewComponent(cfgFile, specFile, ignoredCheckers)
	case consts.ComponentNameNCCL:
		return nccl.NewComponent(cfgFile)
	default:
		return nil, fmt.Errorf("invalid component name: %s", componentName)
	}
}

func RunComponentCheck(ctx context.Context, componentName, cfg, specFile string, ignoredCheckers []string, timeout time.Duration) (*CheckResults, error) {
	comp, err := newComponent(componentName, cfg, specFile, ignoredCheckers)
	if err != nil {
		logrus.WithField("component", componentName).Errorf("failed to create component: %v", err)
		return nil, err
	}

	result, err := common.RunHealthCheckWithTimeout(ctx, timeout, comp.Name(), comp.HealthCheck)
	if err != nil {
		logrus.WithField("component", componentName).Error(err)
		return nil, err
	}

	info, err := comp.LastInfo()
	if err != nil {
		logrus.WithField("component", componentName).Errorf("get to ge the LastInfo: %v", err)
		return nil, err
	}
	return &CheckResults{
		component: comp,
		result:    result,
		info:      info,
	}, nil
}

func PrintCheckResults(summaryPrint bool, checkResult *CheckResults) {
	passed := checkResult.component.PrintInfo(checkResult.info, checkResult.result, summaryPrint)
	StatusMutex.Lock()
	ComponentStatuses[checkResult.component.Name()] = passed
	StatusMutex.Unlock()
}
