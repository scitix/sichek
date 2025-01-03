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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	commonCfg "github.com/scitix/sichek/config"
)

// MockPCIEACSChecker 用于替代实际的 `executeSetpciCommand` 以便在测试中提供 mock 行为
type MockPCIEACSChecker struct {
	PCIEACSChecker
	mockExecuteSetpciCommand func(bdf string, flag string) (string, error)
}

func (m *MockPCIEACSChecker) executeSetpciCommand(bdf string, flag string) (string, error) {
	if m.mockExecuteSetpciCommand != nil {
		return m.mockExecuteSetpciCommand(bdf, flag)
	}
	return "", nil
}

func TestPCIEACSChecker_Check(t *testing.T) {
	// Mock 的 Spec 配置，设定 ACS 期望值
	spec := &config.InfinibandHCASpec{
		SoftwareDependencies: config.SoftwareDependencies{
			PcieACS: "0000", // 模拟的期望 ACS 值
		},
	}

	// 创建 Mock Checker 实例
	checker := &MockPCIEACSChecker{
		PCIEACSChecker: PCIEACSChecker{
			spec: spec,
		},
	}

	// 测试用例
	tests := []struct {
		name               string
		mockSetpciOutput   []string // 模拟 setpci 命令输出
		mockSetpciErrors   []error  // setpci 命令执行错误
		ibDeviceCount      int      // 模拟 IB 设备数量
		expectedStatus     string   // 预期状态
		expectedLevel      string   // 预期等级
		expectedDetail     string   // 期望的详细信息
		expectedSuggestion string   // 期望的建议信息
		expectError        bool     // 是否预期出错
	}{
		{
			name:               "No IB devices found",
			mockSetpciOutput:   nil,
			mockSetpciErrors:   nil,
			ibDeviceCount:      0,
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[checker.name].Level,
			expectedDetail:     config.NOIBFOUND,
			expectedSuggestion: "",
			expectError:        true,
		},
		{
			name: "All devices ACS compliant",
			mockSetpciOutput: []string{
				"0000",
				"0000",
			},
			mockSetpciErrors:   nil,
			ibDeviceCount:      2,
			expectedStatus:     commonCfg.StatusNormal,
			expectedLevel:      commonCfg.LevelInfo,
			expectedDetail:     "",
			expectedSuggestion: "",
			expectError:        false,
		},
		{
			name: "Some devices ACS non-compliant",
			mockSetpciOutput: []string{
				"0000",
				"1234",
			},
			mockSetpciErrors:   nil,
			ibDeviceCount:      2,
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[checker.name].Level,
			expectedDetail:     "bdf:device2 need to disable acs, curr:0000,1234",
			expectedSuggestion: "use shell cmd \"for i in $(Ispci | cut -f 1 -d \\\"\"); do setpci-v -s $i ecap_acs+6.v=0; done\" disable acs",
			expectError:        false,
		},
		{
			name: "Execution failure for setpci command",
			mockSetpciOutput: []string{
				"0000",
			},
			mockSetpciErrors: []error{
				errors.New("setpci: command not found"),
			},
			ibDeviceCount:      1,
			expectedStatus:     commonCfg.StatusNormal,
			expectedLevel:      commonCfg.LevelInfo,
			expectedDetail:     "",
			expectedSuggestion: "",
			expectError:        false,
		},
	}

	// 遍历测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock `executeSetpciCommand` 的行为
			checker.mockExecuteSetpciCommand = func(bdf string, flag string) (string, error) {
				index := strings.TrimPrefix(bdf, "device")
				idx := 0
				if index != "" {
					fmt.Sscanf(index, "%d", &idx)
				}
				if idx < len(tt.mockSetpciOutput) {
					return tt.mockSetpciOutput[idx], tt.mockSetpciErrors[idx]
				}
				return "", nil
			}

			// 创建 Mock IB 设备
			var ibHardwareInfo []collector.IBHardWareInfo
			for i := 0; i < tt.ibDeviceCount; i++ {
				ibHardwareInfo = append(ibHardwareInfo, collector.IBHardWareInfo{
					HCAType: "mlx5",
					IBDev:   fmt.Sprintf("device%d", i+1),
					PCIEBDF: fmt.Sprintf("device%d", i+1),
				})
			}

			data := &collector.InfinibandInfo{
				IBHardWareInfo: ibHardwareInfo,
				IBSoftWareInfo: collector.IBSoftWareInfo{},
			}

			// 调用 Check 方法
			ctx := context.Background()
			result, err := checker.Check(ctx, data)

			// 验证错误状态
			if (err != nil) != tt.expectError {
				t.Fatalf("unexpected error state, got error: %v, expectedError: %v", err, tt.expectError)
			}

			// 验证结果
			if result != nil {
				if result.Status != tt.expectedStatus {
					t.Errorf("unexpected status, got: %s, want: %s", result.Status, tt.expectedStatus)
				}
				if result.Level != tt.expectedLevel {
					t.Errorf("unexpected level, got: %s, want: %s", result.Level, tt.expectedLevel)
				}
				if result.Detail != tt.expectedDetail {
					t.Errorf("unexpected detail, got: %s, want: %s", result.Detail, tt.expectedDetail)
				}
				if result.Suggestion != tt.expectedSuggestion {
					t.Errorf("unexpected suggestion, got: %s, want: %s", result.Suggestion, tt.expectedSuggestion)
				}
			}
		})
	}
}
