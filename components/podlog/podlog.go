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
package podlog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	filter "github.com/scitix/sichek/components/common/eventfilter"
	"github.com/scitix/sichek/components/podlog/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/k8s"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.PodLogUserConfig
	eventRule     *config.PodLogEventRule
	cfgMutex      sync.Mutex

	podResourceMapper *k8s.PodResourceMapper

	cacheMtx          sync.RWMutex
	cacheInfoBuffer   []common.Info
	cacheResultBuffer []*common.Result
	currIndex         int64
	cacheSize         int64

	service *common.CommonService
}

var (
	ncclComponent     common.Component
	ncclComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	ncclComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component nccl: %v", r)
			}
		}()
		ncclComponent, err = newComponent(cfgFile, specFile)
	})
	return ncclComponent, err
}

func newComponent(cfgFile string, specFile string) (comp common.Component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	ncclCfg := &config.PodLogUserConfig{}
	err = common.LoadUserConfig(cfgFile, ncclCfg)
	if err != nil || ncclCfg.NCCL == nil {
		logrus.WithField("component", "podlog").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewNcclComponent get user config failed")
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "podlog").Errorf("failed to NewComponent: %v", err)
		return nil, err
	}

	podResourceMapper := k8s.NewPodResourceMapper()
	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNamePodLog,
		cfg:           ncclCfg,
		eventRule:     eventRules,

		podResourceMapper: podResourceMapper,

		cacheResultBuffer: make([]*common.Result, ncclCfg.NCCL.CacheSize),
		cacheInfoBuffer:   make([]common.Info, ncclCfg.NCCL.CacheSize),
		currIndex:         0,
		cacheSize:         ncclCfg.NCCL.CacheSize,
	}
	component.service = common.NewCommonService(ctx, ncclCfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	allFiles, err := c.GetRunningPodFilePaths(c.eventRule.DirPath)
	if err != nil {
		logrus.WithError(err).Errorf("failed to walkdir in %s", c.eventRule.DirPath)
		return nil, err
	}

	joinedLogFiles := strings.Join(allFiles, ",")
	for _, eventChecker := range c.eventRule.EventCheckers {
		if eventChecker != nil {
			eventChecker.LogFile = joinedLogFiles
		}
	}
	filterPointer, err := filter.NewEventFilter(consts.ComponentNamePodLog, c.eventRule.EventCheckers, 0)
	if err != nil {
		logrus.WithError(err).Error("failed to create filter in NCCLCollector")
		return nil, err
	}
	defer filterPointer.Close()

	result := filterPointer.Check()
	if result == nil {
		logrus.WithField("component", "podlog").WithError(err).Error("failed to Collect(ctx)")
		return nil, err
	}
	result.Item = c.componentName
	for _, checkerResult := range result.Checkers {
		if checkerResult.Status == consts.StatusAbnormal {
			fileNameList := strings.Split(checkerResult.Device, ",")
			podNameList := make([]string, 0, len(fileNameList))
			podNameMap := make(map[string]struct{})
			for _, fileName := range fileNameList {
				podName, _ := getPodNameFromFileName(fileName)
				if _, exists := podNameMap[podName]; exists {
					continue // Skip if podName already exists
				}
				podNameMap[podName] = struct{}{} // Add podName to map
				podNameList = append(podNameList, podName)
			}
			checkerResult.Device = strings.Join(podNameList, ",")
		}
	}
	c.cacheMtx.Lock()
	c.cacheResultBuffer[c.currIndex%c.cacheSize] = result
	c.cacheInfoBuffer[c.currIndex%c.cacheSize] = nil
	c.currIndex++
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "podlog").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "podlog").Infof("Health Check PASSED")
	}

	return result, nil
}

func (c *component) GetRunningPodFilePaths(dir string) ([]string, error) {
	deviceToPodMap, err := c.podResourceMapper.GetDeviceToPodMap()
	if err != nil {
		logrus.WithField("component", "podlog").WithError(err).Error("failed to GetDeviceToPodMap")
		return nil, err
	}
	runningPodSet := make(map[string]struct{})
	for _, podInfo := range deviceToPodMap {
		runningPodSet[podInfo.PodName] = struct{}{}
	}
	// Walk through the directory to find valid pod log files
	runningPodFilePaths := make([]string, 0)
	allFiles := make(map[string]struct{})

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			logrus.WithField("component", "podlog").WithError(walkErr).Errorf("skip dir %s", path)
			return nil // Skip the path if there is an error
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".gz") {
			return nil // Skip gzipped files
		}
		if _, exists := allFiles[path]; exists {
			return nil // Skip if the file has already been processed
		}
		allFiles[path] = struct{}{}
		if !strings.HasSuffix(path, ".log") {
			logrus.WithField("component", "podlog").Debugf("skip file %s, not a log file", path)
			return nil // Skip if the file is not a log file
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			logrus.WithField("component", "podlog").WithError(err).Errorf("failed to get absolute path for %s", path)
			return nil // Skip if we can't get the absolute path
		}
		podName, err := getPodNameFromFileName(absPath)
		if err != nil {
			logrus.WithError(err).Warnf("cannot extract podName from %s", absPath)
			return nil
		}
		if _, exists := runningPodSet[podName]; exists {
			runningPodFilePaths = append(runningPodFilePaths, absPath)
		}
		return nil
	})
	if err != nil {
		logrus.WithField("component", "podlog").WithError(err).Errorf("failed to walk dir %s", dir)
		return nil, fmt.Errorf("failed to walk dir %s: %w", dir, err)
	}
	return runningPodFilePaths, err
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheResultBuffer, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheInfoBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheResultBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheResultBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) LastInfo() (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	var info common.Info
	if c.currIndex == 0 {
		info = c.cacheInfoBuffer[c.cacheSize-1]
	} else {
		info = c.cacheInfoBuffer[c.currIndex-1]
	}
	return info, nil
}

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	config, ok := cfg.(*config.PodLogUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for nccl")
	}
	c.cfg = config
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
	ncclEvents := make(map[string]string)

	checkerResults := result.Checkers
	for _, result := range checkerResults {
		switch result.Name {
		case "NCCLTimeoutChecker":
			if result.Status == consts.StatusAbnormal {
				ncclEvents["NCCLTimeoutChecker"] = fmt.Sprintf("%sDetect NCCLTimeout in pod %s%s", consts.Red, result.Device, consts.Reset)
			}
		}
	}
	utils.PrintTitle("PodLog", "-")
	checkAllPassed := true
	for _, checkerResult := range checkerResults {
		switch checkerResult.Name {
		case "NCCLTimeoutChecker":
			if result.Status == consts.StatusAbnormal {
				checkAllPassed = false
				fmt.Printf("\t%sNCCL timeout detected%s\n", consts.Red, consts.Reset)
				fmt.Printf("\tAffected Pods  : %s\n", checkerResult.Device)
				fmt.Printf("\tTimeout Count  : %s\n", checkerResult.Curr)
				if checkerResult.Suggestion != "" {
					fmt.Printf("\tSuggestion     : %s\n", checkerResult.Suggestion)
				}
			}
		}
	}
	if checkAllPassed {
		fmt.Printf("%sNo PodLog Error detected%s\n", consts.Green, consts.Reset)
	}
	return checkAllPassed
}

func getPodNameFromFileName(fileName string) (string, error) {
	paths := strings.Split(fileName, "/")
	if len(paths) < 5 {
		return "", fmt.Errorf("invalid fileName format=%s, expected at least four '/' character", fileName)
	}
	parts := strings.Split(paths[4], "_")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid fileName format=%s, expected at least one '_' character", fileName)
	}
	return parts[1], nil
}
