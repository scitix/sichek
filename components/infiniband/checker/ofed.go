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
	"fmt"
	"regexp"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type IBOFEDChecker struct {
	id          string
	name        string
	spec        config.InfinibandSpec
	description string
}

func NewIBOFEDChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBOFEDChecker{
		id:          consts.CheckerIDInfinibandOFED,
		name:        config.CheckIBOFED,
		spec:        *specCfg,
		description: "check the rdma ofed",
	}, nil
}

func (c *IBOFEDChecker) Name() string {
	return c.name
}

func (c *IBOFEDChecker) Description() string {
	return c.description
}

func (c *IBOFEDChecker) GetSpec() common.CheckerSpec {
	return nil
}

// parseOFEDVersion extracts major and minor versions from both standard and internal OFED version strings.
// Examples:
//
//	MLNX_OFED_LINUX-5.9-0.5.6.0   -> major=5.9,   minor=0.5.6.0
//	OFED-internal-23.10-1.1.9     -> major=23.10, minor=1.1.9
func parseOFEDVersion(ofed string) (major string, minor string, err error) {
	// handle both prefixes: MLNX_OFED_LINUX / OFED-internal
	re := regexp.MustCompile(`^(?:MLNX_OFED_LINUX|OFED-internal)-(\d+\.\d+)-([\d\.]+)$`)
	matches := re.FindStringSubmatch(ofed)
	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid OFED version format: %s", ofed)
	}

	major = matches[1] // first part, like 5.9 or 23.10
	minor = matches[2] // second part, like 0.5.6.0 or 1.1.9
	return major, minor, nil
}

func checkOFEDFormat(spec string, curr string) (bool, error) {
	_, _, err1 := parseOFEDVersion(spec)
	_, _, err2 := parseOFEDVersion(curr)

	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("invalid OFED version format: %v | %v", err1, err2)
	}
	return true, nil
}

// checkOFEDVersion validates if the given OFED version meets the requirements.
func checkOFEDVersion(spec string, curr string) (bool, error) {
	var operator string
	var specVersion string
	// Extract operator and version from the spec string
	switch {
	case strings.HasPrefix(spec, ">="):
		operator = ">="
		specVersion = strings.TrimPrefix(spec, ">=")
	case strings.HasPrefix(spec, ">"):
		operator = ">"
		specVersion = strings.TrimPrefix(spec, ">")
	case strings.HasPrefix(spec, "=="):
		operator = "=="
		specVersion = strings.TrimPrefix(spec, "==")
	default:
		operator = "=="
		specVersion = spec
	}

	pass, err := checkOFEDFormat(specVersion, curr)
	if !pass || err != nil {
		return false, err
	}
	specMajor, specMinor, err := parseOFEDVersion(specVersion)
	if err != nil {
		return false, err
	}
	currMajor, currMinor, err := parseOFEDVersion(curr)
	if err != nil {
		return false, err
	}

	// Condition 1: Major version
	if !common.CompareVersion(operator+specMajor, currMajor) {
		return false, nil
	}

	// Condition 2: Minor version
	if !common.CompareVersion(operator+specMinor, currMinor) {
		return false, nil
	}

	return true, nil
}

func (c *IBOFEDChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	hwInfoLen := len(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	if hwInfoLen == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	curr := infinibandInfo.IBSoftWareInfo.OFEDVer
	// 如果返回driver version，且系统文件下有值，说明驱动正常加载，直接返回驱动版本
	if strings.Contains(curr, "core") {
		logrus.WithField("component", "infiniband").Infof("OFED version not get from rdma-core, but found IB PF devices, using rdma-core version")
		result.Curr = strings.TrimPrefix(curr, "rdma_core:")
		result.Status = consts.StatusNormal
		result.Detail = fmt.Sprintf("current rdma core: %s", curr)
		result.Suggestion = "check the rdma-core installation"
		return &result, nil
	}
	logrus.WithField("component", "infiniband").Infof("Current OFED version from rdma-core: %s", curr)
	spec := c.spec.IBSoftWareInfo.OFEDVer
	infinibandInfo.RLock()
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		if _, ok := c.spec.HCAs[hwInfo.BoardID]; !ok {
			logrus.Warnf("HCA %s not found in spec, skipping %s", hwInfo.BoardID, c.name)
			continue
		}
		hca := c.spec.HCAs[hwInfo.BoardID]
		if hca.Hardware.OFEDVer != "" {
			spec = hca.Hardware.OFEDVer
			logrus.Warnf("use the IB device's OFED spec to check the system OFED version")
		}
	}
	infinibandInfo.RUnlock()
	pass, err := checkOFEDVersion(spec, curr)
	if !pass || err != nil {
		result.Status = consts.StatusAbnormal
		if err == nil {
			result.Detail = fmt.Sprintf("OFED version mismatch, expected:%s  current:%s", spec, curr)
		} else {
			result.Detail = fmt.Sprintf("%s, expected:%s  current:%s", err, spec, curr)
		}
		result.Suggestion = "update the OFED version"
	}

	result.Curr = curr
	result.Spec = spec

	return &result, nil
}
