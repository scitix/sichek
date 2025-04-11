package component

import (
	"context"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"

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

func RunComponentCheck(ctx context.Context, comp common.Component, cfg, specFile string, ignoredCheckers []string, timeout time.Duration) (*CheckResults, error) {
	result, err := common.RunHealthCheckWithTimeout(ctx, timeout, comp.Name(), comp.HealthCheck)
	if err != nil {
		logrus.WithField("component", comp.Name()).Error(err) // Updated to use comp.Name()
		return nil, err
	}

	info, err := comp.LastInfo()
	if err != nil {
		logrus.WithField("component", comp.Name()).Errorf("get to ge the LastInfo: %v", err) // Updated to use comp.Name()
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
