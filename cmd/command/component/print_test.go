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
	"testing"
	"time"

	"github.com/scitix/sichek/components/cpu"
	"github.com/scitix/sichek/components/nvidia"

	"github.com/sirupsen/logrus"
)

func TestCPU_HealthCheckPrint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	component, err := cpu.NewComponent("")
	if err != nil {
		t.Fatalf("failed to create component: %v", err)
	}
	if err != nil {
		logrus.WithField("components", "cpu").Error("fail to Create cpu Components")
	}
	result, err := component.HealthCheck(ctx)
	if err != nil {
		logrus.WithField("component", component.Name()).Error(err)
		return
	}

	info, _ := component.LastInfo(ctx)
	PrintSystemInfo(info, result, true)
}

func TestNvidia_HealthCheckPrint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	component, err := nvidia.NewComponent("", "", nil)
	if err != nil {
		t.Fatalf("failed to create component: %v", err)
	}
	if err != nil {
		logrus.WithField("components", "Nvidia").Error("fail to Create Nvidia Components")
	}
	result, err := component.HealthCheck(ctx)
	if err != nil {
		logrus.WithField("component", component.Name()).Error(err)
		return
	}

	info, _ := component.LastInfo(ctx)
	PrintNvidiaInfo(info, result, true)
}
