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
	"encoding/json"
	"time"
)

const (
	OutcomePass    = "pass"
	OutcomeFail    = "fail"
	OutcomeSkip    = "skip"
	OutcomeEnvErr  = "env_err"
	OutcomeExecErr = "exec_err"
	OutcomeTimeout = "timeout"
)

type GpuProbeResult struct {
	Index      int    `json:"index"`
	BDF        string `json:"bdf"`
	State      string `json:"state"`   // "idle" | "busy"
	Outcome    string `json:"outcome"` // OutcomeXxx
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Detail     string `json:"detail"`
}

type GpuProbeInfo struct {
	Time   time.Time        `json:"time"`
	PerGPU []GpuProbeResult `json:"per_gpu"`
}

func (o *GpuProbeInfo) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}
