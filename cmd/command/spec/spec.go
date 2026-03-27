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

// Package spec provides CLI-layer helpers for resolving sichek spec/config files.
// All heavy lifting (path discovery, OSS download, backup, tracing) is handled
// by common.EnsureSpecFile; this package exposes intent-revealing wrappers.
package spec

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

// EnsureCfgFile ensures the user-config YAML is available locally.
// Derives a cluster-specific name (e.g. "taihua_user_config.yaml") when empty,
// then falls back to DefaultUserCfgName.
func EnsureCfgFile(configName string) (string, error) {
	return common.EnsureSpecFile(configName, consts.DefaultUserCfgName)
}

// EnsureSpecFile ensures the hardware-spec YAML is available locally.
// Derives a cluster-specific name (e.g. "taihua_spec.yaml") when empty,
// then falls back to DefaultSpecCfgName.
func EnsureSpecFile(specName string) (string, error) {
	return common.EnsureSpecFile(specName, consts.DefaultSpecCfgName)
}
