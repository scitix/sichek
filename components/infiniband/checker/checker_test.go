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
	"encoding/json"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/config"
	"github.com/scitix/sichek/config/hca"
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/consts"
)

func TestIbChecker_Check(t *testing.T) {
	cfg := &infiniband.InfinibandConfig{}
	err := config.DefaultComponentConfig(consts.ComponentNameInfiniband, cfg, consts.DefaultBasicCfgName)
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}
	specCfg := &infiniband.InfinibandSpec{}
	err = config.DefaultComponentConfig(consts.ComponentNameInfiniband, specCfg, consts.DefaultSpecCfgName)
	if err != nil {
		t.Fatalf("failed to load default sepc config: %v", err)
	}
	hcaSpec := &hca.HCASpec{}
	err = config.DefaultComponentConfig(consts.ComponentNameHCA, hcaSpec, consts.DefaultSpecCfgName)
	if err != nil {
		t.Fatalf("failed to load default hca spec config: %v", err)
	}
	err = specCfg.LoadHCASpec(hcaSpec)
	if err != nil {
		t.Fatalf("failed to load hca spec config: %v", err)
	}
	clusterSpec, err := specCfg.GetClusterInfinibandSpec()
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}

	// Create a new AppClocksChecker
	checkers, err := NewCheckers(cfg, clusterSpec)
	if err != nil {
		t.Fatalf("failed to NewCheckers: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Run the Check method
	var collector collector.InfinibandInfo
	ibInfo := collector.GetIBInfo()
	for _, checker := range checkers {
		result, err := checker.Check(ctx, ibInfo)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Status != "normal" {
			t.Errorf("expected status 'normal', got %v", result.Status)
		}
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("Failed to marshal config to JSON: %v", err)
		}
		t.Logf("checker %v result: %v", checker.Name(), string(jsonData))
	}
}
