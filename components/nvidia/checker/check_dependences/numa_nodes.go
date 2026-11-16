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
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// Overridable in tests.
var (
	numaSysfsRoot = "/sys/devices/system/node"
	numaCheckArch = runtime.GOARCH
)

var numaNodeDirRe = regexp.MustCompile(`^node[0-9]+$`)

type NumaNodesChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewNumaNodesChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &NumaNodesChecker{
		name: config.NumaNodesCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *NumaNodesChecker) Name() string {
	return c.name
}

func countNumaNodes(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if numaNodeDirRe.MatchString(e.Name()) {
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("no NUMA node entries under %s", root)
	}
	return n, nil
}

// Check compares the OS-visible NUMA node count against the expected count from spec.
func (c *NumaNodesChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	_, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}
	result := config.GPUCheckItems[config.NumaNodesCheckerName]

	// Grace (GB200/GB300) exposes GPU HBM as NUMA nodes, so the count is meaningless there.
	if numaCheckArch != "amd64" {
		result.Detail = fmt.Sprintf("skipped on %s", numaCheckArch)
		return &result, nil
	}

	expected := 0
	if c.cfg != nil {
		expected = c.cfg.Dependence.NumaNodes
	}
	if expected <= 0 {
		result.Detail = "skipped: numa_nodes not set in spec"
		return &result, nil
	}

	actual, err := countNumaNodes(numaSysfsRoot)
	if err != nil {
		return nil, fmt.Errorf("count NUMA nodes: %w", err)
	}

	result.Spec = strconv.Itoa(expected)
	result.Curr = strconv.Itoa(actual)
	if actual != expected {
		logrus.WithFields(logrus.Fields{
			"checker":  c.Name(),
			"expected": expected,
			"curr":     actual,
		}).Errorf("NUMA node count mismatch")
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("NUMA nodes: expected %d, got %d", expected, actual)
		result.Suggestion = fmt.Sprintf("Check BIOS Sub-NUMA Clustering (SNC) / AMD NPS setting; this GPU model expects %d NUMA node(s). A reboot is required after the change. If this layout is intended, change nvidia dependence.numa_nodes in the spec or add numa_nodes to ignored_checkers.", expected)
		return &result, nil
	}
	result.Detail = fmt.Sprintf("NUMA nodes: %d", actual)
	result.Suggestion = ""
	return &result, nil
}
