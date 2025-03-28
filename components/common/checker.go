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
package common

import (
	"context"
	"strings"
)

type Checker interface {
	Name() string
	GetSpec() CheckerSpec

	/**
	 * Check the status of the component
	 * @param ctx context.Context
	 * @param data any
	 * @return *CheckerResult
	 * @return error nil for valid result, error for invalid result
	 */
	Check(ctx context.Context, data any) (*CheckerResult, error)
}

func CompareVersion(spec, version string) bool {
	spec = strings.TrimPrefix(spec, "spec")
	version = strings.TrimPrefix(version, "spec")

	specParts := strings.Split(spec, ".")
	versionParts := strings.Split(version, ".")

	// 处理 `*` 通配符
	for i := range specParts {
		if specParts[i] == "*" {
			break // 只对比到 `*` 之前的部分
		}
		if i >= len(versionParts) || specParts[i] != versionParts[i] {
			return false
		}
	}
	return true
}
