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
	"github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

type Service interface {
	Run(ctx context.Context)
	Status(ctx context.Context) (interface{}, error)
	Metrics(ctx context.Context, since time.Time) (interface{}, error)
	Stop() error
}

type DaemonService struct {
	cfg *config.Config

	ctx    context.Context
	cancel context.CancelFunc

	components           map[string]common.Component
	componentsLock       sync.RWMutex
	componentsStatus     map[string]bool
	componentsStatusLock sync.RWMutex
	componentResults     map[string]<-chan *common.Result
	componentsResultLock sync.RWMutex

	notifier Notifier
}

func NewService(ctx context.Context, cfg *config.Config, annoKey string) (s Service, err error) {
	cctx, ccancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			ccancel()
		}
	}()

	notifier, err := NewNotifier(ctx, annoKey)
	if err != nil {
		logrus.WithField("daemon", "new").Errorf("create notifier failed: %v", err)
		return nil, err
	}

	daemon_service := &DaemonService{
		ctx:              cctx,
		cancel:           ccancel,
		cfg:              cfg,
		components:       make(map[string]common.Component),
		componentsStatus: make(map[string]bool),
		componentResults: make(map[string]<-chan *common.Result),
		notifier:         notifier,
	}

	for component_name, enabled := range cfg.Components {
		if !enabled {
			continue
		}

		var component common.Component
		var err error
		switch component_name {
		case config.ComponentNameGpfs:
			component, err = gpfs.NewGpfsComponent("")
		case config.ComponentNameCPU:
			component, err = cpu.NewComponent("")
		case config.ComponentNameInfiniband:
			component, err = infiniband.NewInfinibandComponent("")
		case config.ComponentNameDmesg:
			component, err = dmesg.NewComponent("")
		case config.ComponentNameHang:
			component, err = hang.NewComponent("")
		case config.ComponentNameNvidia:
			component, err = nvidia.NewComponent("", []string{})
		case config.ComponentNameNCCL:
			component, err = nccl.NewComponent("")
		default:
			err = fmt.Errorf("invalid component_name: %s", component_name)
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		daemon_service.components[component_name] = component
	}

	return daemon_service, nil
}

func (d *DaemonService) Run(ctx context.Context) {
	d.componentsLock.Lock()
	d.componentsResultLock.Lock()

	for component_name := range d.cfg.Components {
		component, exist := d.components[component_name]
		if !exist {
			continue
		}
		resultChan := component.Start(d.ctx)
		d.componentResults[component_name] = resultChan
	}
	d.componentsLock.Unlock()
	d.componentsResultLock.Unlock()

	for component_name, resultChan := range d.componentResults {
		go func() {
			d.componentsStatusLock.Lock()
			d.componentsStatus[component_name] = d.components[component_name].Status()
			d.componentsStatusLock.Unlock()

			for {
				select {
				case <-d.ctx.Done():
					return
				case result, ok := <-resultChan:
					if !ok {
						logrus.WithField("daemon", "run").Infof("component %s result channel has closed", component_name)
						return
					}
					err := d.notifier.SetNodeAnnotation(d.ctx, result)
					if err != nil {
						logrus.WithField("daemon", "run").Errorf("set node annotation failed: %v", err)
					}
				}
			}
		}()
	}
}

func (d *DaemonService) Status(ctx context.Context) (interface{}, error) {
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
