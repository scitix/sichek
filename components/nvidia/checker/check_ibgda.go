/*
Copyright 2024 The Scitix Authors.
...
*/
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

type IBGDAChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewIBGDAChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &IBGDAChecker{
		name: config.IBGDACheckerName,
		cfg:  cfg,
	}, nil
}

func (c *IBGDAChecker) Name() string {
	return c.name
}

func (c *IBGDAChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.IBGDACheckerName]
	params := nvidiaInfo.IbgdaEnable
	if params == nil {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.Detail = "Failed to retrieve driver parameters"
		return &result, nil
	}

	var errorDetails []string

	if nvidiaInfo.IbgdaConfigCount > 1 {
		errorDetails = append(errorDetails, fmt.Sprintf("Duplicate or multiple IBGDA configurations detected in modprobe.d (count: %d)", nvidiaInfo.IbgdaConfigCount))
	}

	if val, ok := params["EnableStreamMemOPs"]; !ok || val != "1" {
		errorDetails = append(errorDetails, fmt.Sprintf("EnableStreamMemOPs is invalid (current: %s)", val))
	}

	registryDwords, hasRegistry := params["RegistryDwords"]
	if !hasRegistry || !strings.Contains(registryDwords, "PeerMappingOverride=1") {
		currentVal := "missing"
		if hasRegistry {
			currentVal = registryDwords
		}
		errorDetails = append(errorDetails, fmt.Sprintf("PeerMappingOverride=1 not found (current RegistryDwords: %s)", currentVal))
	}

	if len(errorDetails) > 0 {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.Detail = "IBGDA check failed:\n" + strings.Join(errorDetails, "\n")
		result.Suggestion = "Clean up /etc/modprobe.d/nvidia.conf. Ensure 'EnableStreamMemOPs=1' and 'PeerMappingOverride=1' are set exactly once."
	} else {
		result.Status = consts.StatusNormal
		result.Detail = "IBGDA is correctly enabled"
	}

	return &result, nil
}
