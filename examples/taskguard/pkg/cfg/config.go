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
package cfg

import (
	"time"
)

type KubeConfig struct {
	CheckStatusPeriod time.Duration `json:",default=5m"`
	ResyncPeriod      time.Duration `json:",default=30m"`
	ConfigFile        string        `json:",default=''"`
}

type FaultToleranceConfig struct {
	CheckStatusPeriod       time.Duration `json:",default=5m"`
	EnableTaskGuardLabel    string        `json:",optional"`
	MaxRetryCount           int           `json:",default=3"`
	SiChekNodeAnnotationKey string        `json:",default=scitix.ai/sicheck"`
	LogCheckerRulesPath     string        `json:",default=etc/log-checker-rules.yaml"`
	LogCheckerLines         int64         `json:",default=1000"`
}

type Config struct {
	KubeConfig           KubeConfig
	FaultToleranceConfig FaultToleranceConfig
}
