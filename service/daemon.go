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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/sirupsen/logrus"
)

type Service interface {
	Run()
	Status() (interface{}, error)
	Metrics(ctx context.Context, since time.Time) (interface{}, error)
	Stop() error
}

type DaemonService struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	components           map[string]common.Component
	componentsLock       sync.RWMutex
	componentsStatus     map[string]bool
	componentsStatusLock sync.RWMutex
	componentResults     map[string]<-chan *common.Result
	node                 string
	notifier             Notifier
}

func NewService(components map[string]common.Component, annoKey string) (s Service, err error) {
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
	hostname, err := os.Hostname()
	if err != nil {
		logrus.WithField("daemon", "new").Errorf("get node name failed: %v", err)
	}
	daemonService := &DaemonService{
		ctx:              ctx,
		cancel:           cancel,
		components:       components,
		componentsStatus: make(map[string]bool),
		componentResults: make(map[string]<-chan *common.Result),
		notifier:         notifier,
		node:             hostname,
	}

	return daemonService, nil
}

func (d *DaemonService) Run() {
	d.componentsLock.Lock()

	for componentName, component := range d.components {
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
			result.Node = d.node
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
