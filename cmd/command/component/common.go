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
