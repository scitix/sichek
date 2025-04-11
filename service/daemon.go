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
package service

import (
	"context"
	"fmt"
	"strings"
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
	"github.com/scitix/sichek/metrics"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

const timeoutDuration = 120 * time.Second

type Service interface {
	Run()
	Status() (interface{}, error)
	Metrics(ctx context.Context, since time.Time) (interface{}, error)
	Stop() error
}

type DaemonService struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	usedComponentsMap    map[string]bool
	components           map[string]common.Component
	componentsLock       sync.RWMutex
	componentsStatus     map[string]bool
	componentsStatusLock sync.RWMutex
	componentResults     map[string]<-chan *common.Result
	componentsResultLock sync.RWMutex
	metrics              *metrics.HealthCheckResMetrics

	notifier Notifier
}

func NewService(cfgFile string, specFile string, usedComponents []string, ignoredComponents []string, annoKey string) (s Service, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	notifier, err := NewNotifier(annoKey)
	if err != nil {
		logrus.WithField("daemon", "new").Errorf("create notifier failed: %v", err)
		return nil, err
	}
	usedComponentsMap := make(map[string]bool)
	if len(usedComponents) == 0 {
		usedComponents = consts.DefaultComponents
	}
	for _, componentName := range usedComponents {
		usedComponentsMap[componentName] = true
	}
	for _, componentName := range ignoredComponents {
		usedComponentsMap[componentName] = false
	}

	daemonService := &DaemonService{
		ctx:               ctx,
		cancel:            cancel,
		usedComponentsMap: usedComponentsMap,
		components:        make(map[string]common.Component),
		componentsStatus:  make(map[string]bool),
		componentResults:  make(map[string]<-chan *common.Result),
		notifier:          notifier,
		metrics:           metrics.NewHealthCheckResMetrics(),
	}

	for componentName, enabled := range usedComponentsMap {
		if !enabled {
			continue
		}
		var component common.Component
		var err error
		switch componentName {
		case consts.ComponentNameGpfs:
			component, err = gpfs.NewGpfsComponent(cfgFile)
		case consts.ComponentNameCPU:
			component, err = cpu.NewComponent(cfgFile)
		case consts.ComponentNameInfiniband:
			component, err = infiniband.NewInfinibandComponent(cfgFile, specFile, nil)
		case consts.ComponentNameDmesg:
			component, err = dmesg.NewComponent(cfgFile)
		case consts.ComponentNameNvidia:
			if !utils.IsNvidiaGPUExist() {
				logrus.Warn("Nvidia GPU is not Exist. Bypassing GPU HealthCheck")
				continue
			}
			component, err = nvidia.NewComponent(cfgFile, specFile, nil)
		case consts.ComponentNameHang:
			if !utils.IsNvidiaGPUExist() {
				logrus.Warn("Nvidia GPU is not Exist. Bypassing Hang HealthCheck")
				continue
			}
			_, err = nvidia.NewComponent(cfgFile, specFile, nil)
			if err != nil {
				logrus.WithField("component", "all").Errorf("Failed to Get Nvidia component, Bypassing HealthCheck")
				continue
			}
			component, err = hang.NewComponent(cfgFile)
		case consts.ComponentNameNCCL:
			component, err = nccl.NewComponent(cfgFile)
		default:
			err = fmt.Errorf("invalid component_name: %s", componentName)
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		daemonService.components[componentName] = component
	}

	return daemonService, nil
}

func (d *DaemonService) Run() {
	d.componentsLock.Lock()

	for componentName := range d.usedComponentsMap {
		component, exist := d.components[componentName]
		if !exist {
			continue
		}
		resultChan := component.Start()
		d.componentResults[componentName] = resultChan
	}
	d.componentsLock.Unlock()

	for componentName, resultChan := range d.componentResults {
		go d.monitorComponent(componentName, resultChan)
	}
}

func (d *DaemonService) monitorComponent(componentName string, resultChan <-chan *common.Result) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("[DaemonService|monitorComponent] panic err is %s\n", err)
		}
	}()
	d.componentsStatusLock.Lock()
	d.componentsStatus[componentName] = d.components[componentName].Status()
	d.componentsStatusLock.Unlock()
	for {
		logrus.WithField("daemon", "run").Infof("start to listen component %s result channel", componentName)
		select {
		case <-d.ctx.Done():
			logrus.WithField("daemon", "run").Warnf("component %s stop listen as d.ctx.Done()", componentName)
			return
		case result, ok := <-resultChan:
			if !ok {
				logrus.WithField("daemon", "run").Infof("component %s result channel has closed", componentName)
				return
			} else {
				logrus.WithField("daemon", "run").Infof("Get component %s result", componentName)
			}
			var err error
			if strings.Contains(result.Checkers[0].Name, "HealthCheckTimeout") {
				err = d.notifier.AppendNodeAnnotation(d.ctx, result)
			} else {
				err = d.notifier.SetNodeAnnotation(d.ctx, result)
			}
			if err != nil {
				logrus.WithField("daemon", "run").Errorf("set node annotation failed: %v", err)
			}
		}
	}
}

func (d *DaemonService) exportTimeoutResolved(componentName string) {
	timeoutResolvedCheckerResult := &common.CheckerResult{
		Name:        fmt.Sprintf("%sTimeout", componentName),
		Description: fmt.Sprintf("component %s did not return a result within %v s", componentName, timeoutDuration),
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   fmt.Sprintf("%sTimeout", componentName),
		Suggestion:  "The Nvidida GPU may be broken, please restart the node",
	}
	timeoutResolvedResult := &common.Result{
		Item:     componentName,
		Status:   consts.StatusNormal,
		Level:    consts.LevelCritical,
		Checkers: []*common.CheckerResult{timeoutResolvedCheckerResult},
		Time:     time.Now(),
	}
	d.metrics.ExportMetrics(timeoutResolvedResult)
}

func (d *DaemonService) Status() (interface{}, error) {
	return d.componentsStatus, nil
}

func (d *DaemonService) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

func (d *DaemonService) Stop() error {
	var err error
	for _, component := range d.components {
		go func() {
			err = component.Stop()
			if err != nil {
				logrus.WithField("daemon", "stop").Errorf("component %s stop failed: %v", component.Name(), err)
			}
		}()
	}
	d.cancel()
	return err
}
