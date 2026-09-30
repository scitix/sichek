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
package config

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type GpuProbeSpecConfig struct {
	GpuProbe *GpuProbeSpec `json:"gpuprobe" yaml:"gpuprobe"`
}

type GpuProbeSpec struct {
	ProbeBinaryPath          string `json:"probe_binary_path" yaml:"probe_binary_path"`
	ProbeTimeoutSec          int    `json:"probe_timeout_sec" yaml:"probe_timeout_sec"`
	KillGraceSec             int    `json:"kill_grace_sec" yaml:"kill_grace_sec"`
	MinFreeMemPct            int    `json:"min_free_mem_pct" yaml:"min_free_mem_pct"`
	MaxGpuUtilPct            int    `json:"max_gpu_util_pct" yaml:"max_gpu_util_pct"`
	SkipIfComputeApps        bool   `json:"skip_if_compute_apps" yaml:"skip_if_compute_apps"`
	SkipMig                  bool   `json:"skip_mig" yaml:"skip_mig"`
	FailConsecutiveThreshold int    `json:"fail_consecutive_threshold" yaml:"fail_consecutive_threshold"`
	FailLevel                string `json:"fail_level" yaml:"fail_level"`
	EnvErrorLevel            string `json:"env_error_level" yaml:"env_error_level"`
}

// valid reports whether a loaded spec has our required fields populated. A
// legacy/stale `gpuprobe:` section with a different schema unmarshals into a
// zero-valued GpuProbeSpec and must be rejected so we fall back to the
// built-in default rather than checking nothing. The nil receiver check
// makes valid() safe to call on a nil *GpuProbeSpec.
func (s *GpuProbeSpec) valid() bool {
	return s != nil && s.ProbeBinaryPath != "" && s.ProbeTimeoutSec > 0 && s.MinFreeMemPct > 0
}

// LoadSpec loads the gpuprobe spec from file; on any failure, or when the
// loaded spec is empty/legacy-shaped, it returns the built-in default. It
// deliberately never writes a spec file to disk (see components/ovs/config
// for the same pattern) to avoid overwriting the production canonical config.
func LoadSpec(file string) (*GpuProbeSpec, error) {
	if file == "" {
		return DefaultSpec(), nil
	}
	var s GpuProbeSpecConfig
	if err := common.LoadSpec(file, &s); err != nil || !s.GpuProbe.valid() {
		logrus.WithField("component", "gpuprobe/spec").Warnf("spec in %s missing/invalid (err=%v), using built-in default", file, err)
		return DefaultSpec(), nil
	}
	return s.GpuProbe, nil
}

func DefaultSpec() *GpuProbeSpec {
	return &GpuProbeSpec{
		ProbeBinaryPath:          "/var/sichek/bin/gpu_probe",
		ProbeTimeoutSec:          30,
		KillGraceSec:             5,
		MinFreeMemPct:            50,
		MaxGpuUtilPct:            10,
		SkipIfComputeApps:        true,
		SkipMig:                  true,
		FailConsecutiveThreshold: 1,
		FailLevel:                consts.LevelCritical,
		EnvErrorLevel:            consts.LevelWarning,
	}
}
