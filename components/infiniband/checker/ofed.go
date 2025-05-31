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
	spec        config.InfinibandSpecItem
	description string
}

func NewIBOFEDChecker(specCfg *config.InfinibandSpecItem) (common.Checker, error) {
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

// parseOFEDVersion extracts major and minor versions from an OFED version string.
func parseOFEDVersion(ofed string) (major string, minor string, err error) {

	// logrus.Infof("parseOFEDVersion: %s %d %d", ofed,len(ofed),len("MLNX_OFED_LINUX-5.9-0.5.6.0"))
	// Regex to extract the version part: "MLNX_OFED_LINUX-X.Y-A.B.C.D"
	re := regexp.MustCompile(`MLNX_OFED_LINUX-(\d+|\*)\.(\d+|\*)-(\d+|\*)\.(\d+|\*)\.(\d+|\*)\.(\d+|\*)`)
	matches := re.FindStringSubmatch(ofed)
	// logrus.Infof("parseOFEDVersion: %s %d", ofed, len(matches))
	if len(matches) < 7 {
		return "", "", fmt.Errorf("invalid OFED version format: %s", ofed)
	}
	major = matches[1] + "." + matches[2]
	minor = matches[3] + "." + matches[4] + "." + matches[5] + "." + matches[6]
	return major, minor, nil
}
func checkOFEDFormat(spec string, curr string) (bool, error) {
	specsplit := strings.Split(spec, "-")
	currsplit := strings.Split(curr, "-")
	if len(specsplit) != 3 || len(currsplit) != 3 {
		return false, fmt.Errorf("invalid OFED version format: %s %s", spec, curr)
	}
	specMajor := strings.Split(specsplit[1], ".")
	specMinor := strings.Split(specsplit[2], ".")
	currMajor := strings.Split(currsplit[1], ".")
	currMinor := strings.Split(currsplit[2], ".")
	if len(specMajor) != 2 || len(specMinor) != 4 || len(currMajor) != 2 || len(currMinor) != 4 {
		return false, fmt.Errorf("invalid OFED version format: %s %s", spec, curr)
	}
	return true, nil
}

// checkOFEDVersion validates if the given OFED version meets the requirements.
func checkOFEDVersion(spec string, curr string) (bool, error) {
	var operator string
	var specVersion string
	// Extract operator and version from the spec string
	if strings.HasPrefix(spec, ">=") {
		operator = ">="
		specVersion = strings.TrimPrefix(spec, ">=")
	} else if strings.HasPrefix(spec, ">") {
		operator = ">"
		specVersion = strings.TrimPrefix(spec, ">")
	} else if strings.HasPrefix(spec, "==") {
		operator = "=="
		specVersion = strings.TrimPrefix(spec, "==")
	} else {
		operator = "==" // Default to "==" if no operator is specified
		specVersion = spec
	}
	pass, err := checkOFEDFormat(spec, curr)
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

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	curr := infinibandInfo.IBSoftWareInfo.OFEDVer
	spec := c.spec.IBSoftWareInfo.OFEDVer
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hca := c.spec.HCAs[hwInfo.BoardID]
		if hca.OFEDVer != "" {
			spec = hca.OFEDVer
			logrus.Warnf("use the IB device's OFED spec to check the system OFED version")
		}
	}
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
