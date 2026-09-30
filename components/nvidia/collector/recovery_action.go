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
package collector

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

// getRecoveryAction returns the driver-reported "GPU Recovery Action" for the
// given GPU index, parsed from `nvidia-smi -q`. Newer drivers (H200/570+) expose
// this field; when the running driver predates it — or the field is absent/N/A —
// the returned string is empty, which callers treat as "no action required".
//
// Any nvidia-smi failure is logged and swallowed so a transient probe error
// never marks an otherwise-healthy GPU unavailable. Crucially, nvidia-smi's
// query path still succeeds on a GPU that is stuck in "reset required": that is
// exactly the state the plain NVML snapshot counters (ECC/remapped rows) miss
// and this field catches.
func getRecoveryAction(index int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := utils.ExecCommand(ctx, "nvidia-smi", "-q", "-i", strconv.Itoa(index))
	if err != nil {
		logrus.WithField("component", "nvidia").Warnf("failed to query GPU recovery action for GPU %d: %v", index, err)
		return ""
	}
	return parseRecoveryAction(string(out))
}

// parseRecoveryAction extracts the value of the "GPU Recovery Action" line from
// nvidia-smi -q text output, e.g. "    GPU Recovery Action : Reset". It returns
// "" when the line is absent (older drivers). Values other than "None"/"N/A"
// (Reset, Reboot, "Drain and Reset", "Drain P2P") mean the GPU needs operator
// intervention before it can run CUDA work again.
func parseRecoveryAction(smiOutput string) string {
	scanner := bufio.NewScanner(strings.NewReader(smiOutput))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "GPU Recovery Action") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// NeedsRecoveryAction reports whether a recovery-action string signals that the
// GPU requires operator action. "", "None", and "N/A" are healthy; everything
// else (case-insensitive match on the healthy set) is not.
func NeedsRecoveryAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "none", "n/a":
		return false
	default:
		return true
	}
}
