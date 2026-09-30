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

// NewCheckers creates all memory checkers.
// expectedCapacityGB of 0 means the capacity check will be skipped at runtime.
func NewCheckers(expectedCapacityGB float64) ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)

	eccUncorrected, err := NewMemoryECCUncorrectedChecker()
	if err != nil {
		return nil, fmt.Errorf("create memory ecc uncorrected checker failed: %v", err)
	}
	checkers = append(checkers, eccUncorrected)

	eccCorrected, err := NewMemoryECCCorrectedChecker(100)
	if err != nil {
		return nil, fmt.Errorf("create memory ecc corrected checker failed: %v", err)
	}
	checkers = append(checkers, eccCorrected)

	capacity, err := NewMemoryCapacityChecker(expectedCapacityGB, 5.0)
	if err != nil {
		return nil, fmt.Errorf("create memory capacity checker failed: %v", err)
	}
	checkers = append(checkers, capacity)

	return checkers, nil
}
