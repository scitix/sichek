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
	"strings"
	"strings"
	"testing"
	"time"

	"github.com/scitix/sichek/components/nvidia/config"
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/systemd"
	"github.com/scitix/sichek/pkg/utils"
)

func TestNVFabricManagerChecker_Check(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// disable perfomance mode for testing
	t.Logf("======test: `systemctl stop nvidia-fabricmanager`=====")
	t.Logf("======test: `systemctl stop nvidia-fabricmanager`=====")
	output, err := utils.ExecCommand(ctx, "systemctl", "stop", "nvidia-fabricmanager")
	if err != nil {
		if strings.Contains(string(output), "nvidia-fabricmanager.service not loaded") ||
			strings.Contains(string(output), "Failed to connect to bus") { // skip for gitlab-ci
			t.Skipf("command `systemctl stop nvidia-fabricmanager`: output= %v, err=%s", string(output), err.Error())
		} else {
			t.Fatalf("failed to stop nvidia-fabricmanager: %v, output: %v", err, string(output))
		}
	}

	t.Logf("======test: `systemctl is-active nvidia-fabricmanager`=====")
		if strings.Contains(string(output), "nvidia-fabricmanager.service not loaded") ||
			strings.Contains(string(output), "Failed to connect to bus") { // skip for gitlab-ci
			t.Skipf("command `systemctl stop nvidia-fabricmanager`: output= %v, err=%s", string(output), err.Error())
		} else {
			t.Fatalf("failed to stop nvidia-fabricmanager: %v, output: %v", err, string(output))
		}
	}

	t.Logf("======test: `systemctl is-active nvidia-fabricmanager`=====")
	is_active, _ := systemd.IsActive("nvidia-fabricmanager")
	if is_active {
		t.Fatalf("unexpected active nvidia-fabricmanager")
	}

	t.Logf("======test: `systemctl status nvidia-fabricmanager`=====")
	t.Logf("======test: `systemctl status nvidia-fabricmanager`=====")
	output, _ = utils.ExecCommand(ctx, "systemctl", "status", "nvidia-fabricmanager")
	t.Logf("nvidia-fabricmanager status: %s", string(output))

	// Run the Check method
	t.Logf("======test: `do NVFabricManagerChecker and expect to start nvidia-fabricmanager`=====")
	t.Logf("======test: `do NVFabricManagerChecker and expect to start nvidia-fabricmanager`=====")
	cfg := &config.NvidiaConfig{}
	checker, err := NewNVFabricManagerChecker(cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create NVFabricManagerChecker: %v", err)
	}

	result, err := checker.Check(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != commonCfg.StatusNormal {
		t.Fatalf("expected status 'normal', got %v", result.Status)
	}
	t.Logf("result: %v", result)
	t.Logf("======test: `systemctl status nvidia-fabricmanager` after NVFabricManagerChecker =====")
	t.Logf("======test: `systemctl status nvidia-fabricmanager` after NVFabricManagerChecker =====")
	output, _ = utils.ExecCommand(ctx, "systemctl", "status", "nvidia-fabricmanager")
	t.Logf("nvidia-fabricmanager status: %s", string(output))
}

func TestIOMMUChecker_Check(t *testing.T) {
	// Create a new IOMMUChecker
	cfg := &config.NvidiaConfig{}
	checker, err := NewIOMMUChecker(cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create IOMMUChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != commonCfg.StatusNormal {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestNvPeerMemChecker_Check(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// rmmod nvidia_peermem for testing
	t.Logf("======test: `rmmod nvidia_peermem`=====")
	t.Logf("======test: `rmmod nvidia_peermem`=====")
	output, err := utils.ExecCommand(ctx, "rmmod", "nvidia_peermem")
	if err != nil {
		if strings.Contains(string(output), "nvidia_peermem") {
			t.Logf("nvidia_peermem is already unloaded")
		} else {
			t.Fatalf("failed to rmmod nvidia_peermem: %v, output: %v", err, string(output))
		}
		if strings.Contains(string(output), "nvidia_peermem") {
			t.Logf("nvidia_peermem is already unloaded")
		} else {
			t.Fatalf("failed to rmmod nvidia_peermem: %v, output: %v", err, string(output))
		}
	}
	// Check if ib_core and nvidia_peermem, and ib_core is using nvidia_peermem
	usingPeermem, _ := utils.IsKernalModuleHolder("ib_core", "nvidia_peermem")
	if usingPeermem {
		t.Fatalf("unexpected usingPeermem")
	}

	// Run the Check method
	// Create a new NvPeerMemChecker
	t.Logf("======test: `do NvPeerMemChecker and expect to load nvidia_peermem`=====")
	t.Logf("======test: `do NvPeerMemChecker and expect to load nvidia_peermem`=====")
	cfg := &config.NvidiaConfig{}
	checker, err := NewNvPeerMemChecker(cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create NvPeerMemChecker: %v", err)
	}

	result, err := checker.Check(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != commonCfg.StatusNormal {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
	t.Logf("result: %+v", result.ToString())
}

func TestPCIeACSChecker_Check(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	acsCapDevices, err := utils.GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(acsCapDevices) != 0 {
		t.Logf("======test: `EnableACS for 2 capable devices`=====")
		batch := 0
		for _, deviceBDF := range acsCapDevices {
			batch++
			result := utils.EnableACS(ctx, deviceBDF)
			if result != nil {
				t.Fatalf("Unexpected error: %v", result)
			}
			if batch == 2 {
				break
			}
		}
		t.Logf("Enable ACS for 2 capable devices for test")

		t.Logf("======test: `GetACSEnabledDevices`=====")
		pcieACS, err := utils.GetACSEnabledDevices(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		t.Logf("Get %d enabled ACS devices", len(pcieACS))
		if len(pcieACS) != 2 {
			t.Fatalf("Got %d devices that ACS is enabled, expected 2", len(pcieACS))
		}

		// Run the Check method
		// Create a new NvPeerMemChecker
		t.Logf("======test: `do PCIeACSChecker and expect to disable ACS online`=====")
		cfg := &config.NvidiaConfig{}
		checker, err := NewPCIeACSChecker(&cfg.Spec)
		if err != nil {
			t.Fatalf("failed to create PCIeACSChecker: %v", err)
		}

		result, err := checker.Check(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != commonCfg.StatusNormal {
			t.Errorf("expected status 'normal', got %v", result.Status)
		}
		t.Logf("result: %+v", result.ToString())
	}

	t.Logf("======test: `do PCIeACSChecker again and expect all ACS are disabled`=====")
	cfg := &config.NvidiaConfig{}
	checker, err := NewPCIeACSChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create PCIeACSChecker: %v", err)
	}

	result, err := checker.Check(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != commonCfg.StatusNormal {
		t.Fatalf("expected status 'normal', got %v", result.Status)
	}
}
