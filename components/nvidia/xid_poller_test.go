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
	"sync"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

func TestNewXidEventPoller(t *testing.T) {
	// Initialize NVML
	nvmlInst := nvml.New()
	ret := nvmlInst.Init()
	if !errors.Is(ret, nvml.SUCCESS) {
		t.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvmlInst.Shutdown()

	ctx := context.Background()
	cfg := &config.NvidiaUserConfig{}
	eventChan := make(chan *common.Result, 1)
	var nvmlMtx sync.RWMutex

	poller, err := NewXidEventPoller(ctx, cfg, nvmlInst, &nvmlMtx, eventChan)
	if err != nil {
		t.Errorf("failed to create XidEventPoller: %v", err)
	}
	if poller == nil {
		t.Error("failed to create XidEventPoller")
	}
}

func TestXidEventPoller_Start(t *testing.T) {
	// Initialize NVML
	nvmlInst := nvml.New()
	ret := nvmlInst.Init()
	if !errors.Is(ret, nvml.SUCCESS) {
		t.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvmlInst.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := &config.NvidiaUserConfig{}
	eventChan := make(chan *common.Result, 1)
	var nvmlMtx sync.RWMutex

	poller, err := NewXidEventPoller(ctx, cfg, nvmlInst, &nvmlMtx, eventChan)
	if err != nil {
		t.Errorf("failed to create XidEventPoller: %v", err)
	}
	if poller == nil {
		t.Error("failed to create XidEventPoller")
	}

	go func() {
		err := poller.Start()
		if err != nil {
			t.Errorf("failed to start XidEventPoller: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)
	cancel()
	time.Sleep(1 * time.Second)
}
