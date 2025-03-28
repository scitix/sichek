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
	"strconv"
	"strings"
)

type Checker interface {
	Name() string

	/**
	 * Check the status of the component
	 * @param ctx context.Context
	 * @param data any
	 * @return *CheckerResult
	 * @return error nil for valid result, error for invalid result
	 */
	Check(ctx context.Context, data any) (*CheckerResult, error)
}

// compareVersions compares two version strings and returns:
// -1 if version < spec
//
//	0 if version == spec
//	1 if version > spec
func compareVersions(version, spec string) int {
	spec = strings.TrimSpace(spec)
	version = strings.TrimSpace(version)
	vParts := strings.Split(version, ".")
	sParts := strings.Split(spec, ".")

	// Ensure both version and spec have the same length by padding with "0"
	for len(vParts) < len(sParts) {
		vParts = append(vParts, "0")
	}
	for len(sParts) < len(vParts) {
		sParts = append(sParts, "0")
	}

	// Compare each part numerically
	for i := 0; i < len(sParts); i++ {
		if sParts[i] == "*" {
			return 0 // The wildcard '*' matches everything after this point
		}
		vNum, _ := strconv.Atoi(vParts[i])
		sNum, _ := strconv.Atoi(sParts[i])
		if vNum > sNum {
			return 1
		} else if vNum < sNum {
			return -1
		}
	}
	return 0
}

// CompareVersion parses the spec and compares it with the given version.
// Supports operators: ">=", ">", "==" and wildcard "*".
func CompareVersion(spec, version string) bool {
	spec = strings.TrimSpace(spec)
	version = strings.TrimSpace(version)

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

	// Compare the version against the spec
	comp := compareVersions(version, specVersion)

	// Determine if the version meets the spec condition
	switch operator {
	case ">=":
		return comp >= 0
	case ">":
		return comp > 0
	case "==":
		return comp == 0
	default:
		return false
	}
}
