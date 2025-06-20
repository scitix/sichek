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

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	hcaConfig "github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/consts"
)

func TestIBPhyStateChecker_Check(t *testing.T) {
	spec := &config.InfinibandSpec{
		HCAs: map[string]*hcaConfig.HCASpec{
			"MT_0000000970": {
				Hardware: collector.IBHardWareInfo{
					IBDev:    "MT_0000000970",
					BoardID:  "MT_0000000970",
					PhyState: "LinkUp",
				},
			},
			"MT_0000001119": {
				Hardware: collector.IBHardWareInfo{
					IBDev:    "MT_0000001119",
					BoardID:  "MT_0000001119",
					PhyState: "LinkUp",
				},
			},
		},
	}

	checker, err := NewIBPhyStateChecker(spec)
	if err != nil {
		t.Fatalf("failed to create IBPhyStateChecker: %v", err)
	}

	ibChecker := checker.(*IBPhyStateChecker)

	tests := []struct {
		name           string
		data           *collector.InfinibandInfo
		expectedStatus string
		expectedLevel  string
		expectedError  bool
	}{
		{
			name: "All devices are LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT_0000001119", BoardID: "MT_0000001119", IBDev: "ib0", PhyState: "LinkUp"},
					{HCAType: "MT_0000000970", BoardID: "MT_0000000970", IBDev: "ib1", PhyState: "LinkUp"},
				},
			},
			expectedStatus: consts.StatusNormal,
			expectedLevel:  consts.LevelCritical,
			expectedError:  false,
		},
		{
			name: "No Infiniband devices found",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus: consts.StatusAbnormal,
			expectedLevel:  config.InfinibandCheckItems[ibChecker.name].Level,
			expectedError:  true,
		},
		{
			name: "One device is not LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT_0000001119", BoardID: "MT_0000001119", IBDev: "ib0", PhyState: "Down"},
					{HCAType: "MT_0000000970", BoardID: "MT_0000000970", IBDev: "ib1", PhyState: "LinkUp"},
				},
			},
			expectedStatus: consts.StatusAbnormal,
			expectedLevel:  config.InfinibandCheckItems[ibChecker.name].Level,
			expectedError:  false,
		},
		{
			name: "Multiple devices are not LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT_0000001119", BoardID: "MT_0000001119", IBDev: "ib0", PhyState: "Down"},
					{HCAType: "MT_0000000970", BoardID: "MT_0000000970", IBDev: "ib1", PhyState: "Init"},
				},
			},
			expectedStatus: consts.StatusAbnormal,
			expectedLevel:  config.InfinibandCheckItems[ibChecker.name].Level,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := ibChecker.Check(ctx, tt.data)

			if (err != nil) != tt.expectedError {
				t.Errorf("unexpected error state, got error=%v, expectedError=%v", err, tt.expectedError)
			}

			if result != nil {
				if result.Status != tt.expectedStatus {
					t.Errorf("unexpected status, got=%s, want=%s", result.Status, tt.expectedStatus)
				}

				if result.Level != tt.expectedLevel {
					t.Errorf("unexpected level, got=%s, want=%s", result.Level, tt.expectedLevel)
				}
			}
		})
	}
}
