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
	"fmt"

	"github.com/scitix/sichek/components/common"
)

func NewCheckers() ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)
	checker, err := NewCPUPerfChecker()
	if err != nil {
		return nil, fmt.Errorf("create cpu perf checker failed: %v", err)
	}
	checkers = append(checkers, checker)
	return checkers, nil
}
