/*
Copyright 2024 The Scitix Authors.
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

type P2PChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewP2PChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &P2PChecker{
		name: config.P2PCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *P2PChecker) Name() string {
	return c.name
}

func (c *P2PChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.P2PCheckerName]
    
	if nvidiaInfo.DeviceCount <= 1 {
		result.Status = consts.StatusNormal
		result.Detail = "Skipped (Single GPU)"
		return &result, nil
	}

	matrix := nvidiaInfo.P2PStatusMatrix
	if matrix == nil || len(matrix) == 0 {
		result.Status = consts.StatusNormal 
		result.Detail = "P2P status unavailable"
		return &result, nil
	}

	var missingLinks []string
	supportedCount := 0

	for key, supported := range matrix {
		if supported {
			supportedCount++
		} else {
            parts := strings.Split(key, "-")
            if len(parts) == 2 {
			    missingLinks = append(missingLinks, fmt.Sprintf("GPU%s -> GPU%s: Not Supported", parts[0], parts[1]))
            }
		}
	}

	if supportedCount == 0 {
		result.Status = consts.StatusNormal 
		result.Detail = "P2P is globally Disabled/Not Supported on this machine"
	} else if len(missingLinks) > 0 {
		result.Status = consts.StatusAbnormal
        if len(missingLinks) > 5 {
		    result.Detail = fmt.Sprintf("Partial P2P failure detected (%d links broken). Examples:\n%s\n...", len(missingLinks), strings.Join(missingLinks[:5], "\n"))
        } else {
            result.Detail = fmt.Sprintf("Partial P2P failure detected:\n%s", strings.Join(missingLinks, "\n"))
        }
	} else {
		result.Status = consts.StatusNormal
		result.Detail = "P2P (Read) is fully supported between all GPUs"
	}

	return &result, nil
}