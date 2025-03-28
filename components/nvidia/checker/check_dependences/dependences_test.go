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
package checker

import (
	"context"
	"testing"
	"time"

	"github.com/scitix/sichek/config/nvidia"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/systemd"
	"github.com/scitix/sichek/pkg/utils"
)

func TestNVFabricManagerChecker_Check(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// disable perfomance mode for testing
	output, err := utils.ExecCommand(ctx, "systemctl", "stop", "nvidia-fabricmanager")
	if err != nil {
		t.Fatalf("failed to stop nvidia-fabricmanager: %v, output: %v", err, output)
	}

	is_active, _ := systemd.IsActive("nvidia-fabricmanager")
	if is_active {
		t.Fatalf("unexpected active nvidia-fabricmanager")
	}

	output, _ = utils.ExecCommand(ctx, "systemctl", "status", "nvidia-fabricmanager")
	t.Logf("nvidia-fabricmanager status: %s", string(output))

	// Run the Check method
	cfg := &nvidia.NvidiaSpecItem{}
	checker, err := NewNVFabricManagerChecker(cfg)
	if err != nil {
		t.Fatalf("failed to create NVFabricManagerChecker: %v", err)
	}

	result, err := checker.Check(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != consts.StatusNormal {
		t.Fatalf("expected status 'normal', got %v", result.Status)
	}
	t.Logf("result: %v", result)
	output, _ = utils.ExecCommand(ctx, "systemctl", "status", "nvidia-fabricmanager")
	t.Logf("nvidia-fabricmanager status: %s", string(output))
}

func TestIOMMUChecker_Check(t *testing.T) {
	// Create a new IOMMUChecker
	cfg := &nvidia.NvidiaSpecItem{}
	checker, err := NewIOMMUChecker(cfg)
	if err != nil {
		t.Fatalf("failed to create IOMMUChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != consts.StatusNormal {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestNvPeerMemChecker_Check(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// rmmod nvidia_peermem for testing
	output, err := utils.ExecCommand(ctx, "rmmod", "nvidia_peermem")
	if err != nil {
		t.Fatalf("failed to rmmod nvidia_peermem: %v, output: %v", err, output)
	}

	// Check if ib_core and nvidia_peermem, and ib_core is using nvidia_peermem
	usingPeermem, _ := utils.IsKernalModuleHolder("ib_core", "nvidia_peermem")
	if usingPeermem {
		t.Fatalf("unexpected usingPeermem")
	}

	// Run the Check method
	// Create a new NvPeerMemChecker
	cfg := &nvidia.NvidiaSpecItem{}
	checker, err := NewNvPeerMemChecker(cfg)
	if err != nil {
		t.Fatalf("failed to create NvPeerMemChecker: %v", err)
	}

	result, err := checker.Check(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != consts.StatusNormal {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
	t.Logf("result: %+v", result.ToString())
}
