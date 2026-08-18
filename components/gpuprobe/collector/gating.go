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
	"context"
	"strconv"
	"strings"
	"time"
)

// defaultSmiTimeout bounds every nvidia-smi invocation so a GPU wedged in a D-state
// driver ioctl (which can hang nvidia-smi itself) cannot block Collect. Because the
// reentrancy latch is acquired before the gating queries run, an unbounded hang here
// would leave the latch stuck and silently stop probing every GPU on the node.
const defaultSmiTimeout = 15 * time.Second

// gpuStat 一张卡的门控原始读数。
type gpuStat struct {
	Index      int
	BDF        string
	UtilPct    int
	FreePct    int
	MigEnabled bool
}

// nvidiaSmi 允许测试注入假命令。默认真跑 nvidia-smi，但经 execBounded 包住，
// 保证即便 nvidia-smi 在 hung GPU 上卡死也会在 defaultSmiTimeout 后返回错误。
var nvidiaSmi = func(ctx context.Context, args ...string) (string, error) {
	return execBounded(ctx, defaultSmiTimeout, "nvidia-smi", args...)
}

// queryGPUs 返回每卡门控读数；无 GPU / nvidia-smi 缺失时返回空切片。
func queryGPUs(ctx context.Context) ([]gpuStat, error) {
	out, err := nvidiaSmi(ctx,
		"--query-gpu=index,pci.bus_id,utilization.gpu,memory.used,memory.total,mig.mode.current",
		"--format=csv,noheader,nounits")
	if err != nil {
		return nil, err
	}
	var stats []gpuStat
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 6 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(f[0]))
		util, _ := strconv.Atoi(strings.TrimSpace(f[2]))
		used, _ := strconv.Atoi(strings.TrimSpace(f[3]))
		total, _ := strconv.Atoi(strings.TrimSpace(f[4]))
		freePct := 0
		if total > 0 {
			freePct = int(float64(total-used) / float64(total) * 100)
		}
		mig := strings.Contains(strings.ToLower(f[5]), "enabled")
		stats = append(stats, gpuStat{
			Index: idx, BDF: shortBDF(strings.TrimSpace(f[1])),
			UtilPct: util, FreePct: freePct, MigEnabled: mig,
		})
	}
	return stats, nil
}

// countComputeApps 返回某卡上的计算进程数。
func countComputeApps(ctx context.Context, idx int) int {
	out, err := nvidiaSmi(ctx, "--query-compute-apps=pid", "--format=csv,noheader", "-i", strconv.Itoa(idx))
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// shortBDF 把 "00000000:BB:00.0" → "bb:00"，便于定位。
func shortBDF(full string) string {
	full = strings.ToLower(strings.TrimSpace(full))
	parts := strings.Split(full, ":")
	if len(parts) >= 3 {
		dev := strings.SplitN(parts[2], ".", 2)[0]
		return parts[1] + ":" + dev
	}
	return full
}
