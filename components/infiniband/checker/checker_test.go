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
	"github.com/scitix/sichek/components/infiniband/config"
	commonCfg "github.com/scitix/sichek/config"
)

func TestIbChecker_Check(t *testing.T) {
	cfg := &config.InfinibandConfig{}
	err := commonCfg.DefaultConfig(commonCfg.ComponentNameInfiniband, cfg)
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}
	clusterSpec := config.GetClusterInfinibandSpec("")
	jsonData, err := json.MarshalIndent(clusterSpec, "", "  ")
	t.Logf("clusterSpec: %v", string(jsonData))
	// Create a new AppClocksChecker
	checkers, err := NewCheckers(cfg, &clusterSpec)
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
