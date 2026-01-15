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
	cfg           *config.PodlogUserConfig
	eventRule     *config.PodlogEventRule
	cfgMutex      sync.Mutex

	podResourceMapper *k8s.PodResourceMapper
	onlyRunningPods   bool  // true: only check running pods; false: check all pods in log_dir
	skipPercent       int64 // skip percent for file reading

	cacheMtx          sync.RWMutex
	cacheInfoBuffer   []common.Info
	cacheResultBuffer []*common.Result
	currIndex         int64
	cacheSize         int64

	service *common.CommonService

	initError error // Track initialization errors with detailed information
}

var (
	podlogComponent     common.Component
	podlogComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string, onlyRunningPods bool, skipPercent int64) (common.Component, error) {
	var err error
	podlogComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component nccl: %v", r)
			}
		}()
		podlogComponent, err = newComponent(cfgFile, specFile, onlyRunningPods, skipPercent)
	})
	return podlogComponent, err
}

func newComponent(cfgFile string, specFile string, onlyRunningPods bool, skipPercent int64) (comp common.Component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNamePodlog,
		cfgMutex:      sync.Mutex{},
	}

	cfg := &config.PodlogUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Podlog == nil {
		logrus.WithField("component", "podlog").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		component.initError = fmt.Errorf("get user config failed: %w", err)
		defaultCfg := &config.PodlogUserConfig{
			Podlog: &config.PodLogConfig{
				QueryInterval: common.Duration{Duration: 10 * time.Second},
				CacheSize:     5,
			},
		}
		component.cfg = defaultCfg
		component.service = common.NewCommonService(ctx, defaultCfg, component.componentName, component.GetTimeout(), component.HealthCheck)
		return component, nil
	}
	component.cfg = cfg

	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "podlog").Errorf("failed to NewComponent: %v", err)
		component.initError = fmt.Errorf("failed to load event rules: %w", err)
		component.service = common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)
		return component, nil
	}

	// if skipPercent is -1, use the value from the config file
	if skipPercent == -1 {
		skipPercent = cfg.Podlog.SkipPercent
		// if config file doesn't have skip_percent or it's 0, use default 100
		if skipPercent == 0 {
			skipPercent = 100
		}
	}

	podResourceMapper := k8s.NewPodResourceMapper()
	component.eventRule = eventRules
	component.podResourceMapper = podResourceMapper
	component.onlyRunningPods = onlyRunningPods
	component.skipPercent = skipPercent
	component.cacheResultBuffer = make([]*common.Result, cfg.Podlog.CacheSize)
	component.cacheInfoBuffer = make([]common.Info, cfg.Podlog.CacheSize)
	component.currIndex = 0
	component.cacheSize = cfg.Podlog.CacheSize

	component.service = common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	if c.initError != nil {
		logrus.WithField("component", "podlog").Errorf("report initError: %v", c.initError)
		checkerResult := &common.CheckerResult{
			Name:        "PodlogInitError",
			Description: "Podlog component initialization failed",
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      c.initError.Error(),
			ErrorName:   "PodlogInitError",
			Suggestion:  "Please check the initialization logs and ensure all dependencies are properly configured",
		}
		result := &common.Result{
			Item:     consts.ComponentNamePodlog,
			Status:   consts.StatusAbnormal,
			Checkers: []*common.CheckerResult{checkerResult},
			Time:     time.Now(),
		}
		return result, nil
	}

	var allFiles []string
	var err error
	if c.onlyRunningPods {
		allFiles, err = c.GetRunningPodFilePaths(c.eventRule.DirPath)
		logrus.WithField("component", "podlog").Debugf("using GetRunningPodFilePaths mode (only running gpu pods)")
	} else {
		allFiles, err = c.GetAllPodFilePaths(c.eventRule.DirPath)
		logrus.WithField("component", "podlog").Debugf("using GetAllPodFilePaths mode (all pods)")
	}
	if err != nil {
		logrus.WithError(err).Errorf("failed to walkdir in %s", c.eventRule.DirPath)
		return nil, err
	}
	if len(allFiles) == 0 {
		logrus.WithField("component", "podlog").Infof("no pod log files found in %s", c.eventRule.DirPath)
		// return a normal result instead of nil
		return &common.Result{
			Item:     c.componentName,
			Time:     time.Now(),
			Status:   consts.StatusNormal,
			Level:    consts.LevelInfo,
			Checkers: []*common.CheckerResult{},
		}, nil
	}
	joinedLogFiles := strings.Join(allFiles, ",")
	for _, eventChecker := range c.eventRule.EventCheckers {
		if eventChecker != nil {
			eventChecker.LogFile = joinedLogFiles
		}
	}
	filterPointer, err := filter.NewEventFilter(consts.ComponentNamePodlog, c.eventRule.EventCheckers, c.skipPercent)
	if err != nil {
		logrus.WithError(err).Error("failed to create filter in podlog component")
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
	// c.cacheInfoBuffer[c.currIndex%c.cacheSize] = nil
	c.currIndex++
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "podlog").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "podlog").Infof("Health Check PASSED")
	}

	return result, nil
}

// walkPodLogFiles walks through the directory and collects pod log file paths.
// filterFunc is called for each valid log file with (absPath, podName, podNameErr).
// If filterFunc returns true, the file is included in the result.
func (c *component) walkPodLogFiles(dir string, filterFunc func(absPath string, podName string, podNameErr error) bool) ([]string, error) {
	filePaths := make([]string, 0)
	allFiles := make(map[string]struct{})

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
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
		// Check if path should be ignored based on configured ignore_namespaces
		if c.shouldIgnoreNamespace(path) {
			return nil
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
		podName, podNameErr := getPodNameFromFileName(absPath)
		if filterFunc(absPath, podName, podNameErr) {
			filePaths = append(filePaths, absPath)
		}
		return nil
	})
	if err != nil {
		logrus.WithField("component", "podlog").WithError(err).Errorf("failed to walk dir %s", dir)
		return nil, fmt.Errorf("failed to walk dir %s: %w", dir, err)
	}
	return filePaths, nil
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

	// Walk through the directory to find valid running pod log files
	return c.walkPodLogFiles(dir, func(absPath string, podName string, podNameErr error) bool {
		if podNameErr != nil {
			logrus.WithError(podNameErr).Warnf("cannot extract podName from %s", absPath)
			return false
		}
		_, exists := runningPodSet[podName]
		return exists
	})
}

func (c *component) GetAllPodFilePaths(dir string) ([]string, error) {
	return c.walkPodLogFiles(dir, func(absPath string, podName string, podNameErr error) bool {
		if podNameErr != nil {
			logrus.WithError(podNameErr).Warnf("cannot extract podName from %s, but still include it", absPath)
			return false
		}
		return true
	})
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheResultBuffer, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	return nil, fmt.Errorf("no info for podlog")
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
	return nil, fmt.Errorf("no info for podlog")
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
	config, ok := cfg.(*config.PodlogUserConfig)
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

	utils.PrintTitle("PodLog", "-")
	checkAllPassed := true
	if result != nil {
		checkerResults := result.Checkers
		for _, checkerResult := range checkerResults {
			if checkerResult.Status == consts.StatusAbnormal {
				checkAllPassed = false
				fmt.Printf("\t%sDetected %s error for %s times in %s%s\n", consts.Red, checkerResult.ErrorName, checkerResult.Curr, checkerResult.Device, consts.Reset)
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

func (c *component) shouldIgnoreNamespace(path string) bool {
	c.cfgMutex.Lock()
	defer c.cfgMutex.Unlock()

	if c.cfg == nil || c.cfg.Podlog == nil {
		return false
	}

	for _, namespace := range c.cfg.Podlog.IgnoreNamespaces {
		if strings.Contains(path, namespace+"_") {
			return true
		}
	}
	return false
}
