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

// NewCheckers returns the rdmaenv checkers. Only the systemd unit is judged; the
// scraped knobs stay passthrough.
func NewCheckers() ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)
	serviceChecker, err := NewServiceChecker()
	if err != nil {
		return nil, fmt.Errorf("create rdma-env-pre service checker failed: %v", err)
	}
	checkers = append(checkers, serviceChecker)
	return checkers, nil
}
