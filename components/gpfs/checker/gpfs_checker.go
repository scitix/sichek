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
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpfs/config"

	"github.com/sirupsen/logrus"
)

func NewCheckers(cfg *config.GpfsEventRule) ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)
	for name, cfg := range cfg.EventCheckers {
		checker, err := NewEventChecker(cfg)
		if err != nil {
			logrus.WithField("component", "gpfs").Errorf("create event checker %s failed: %v", name, err)
			return nil, err
		}
		checkers = append(checkers, checker)
	}
	for _, name := range cfg.XStorHealthCheckers {
		checker, err := NewXStorHealthChecker(cfg, name)
		if err != nil {
			logrus.WithField("component", "gpfs").Errorf("create xstorHealth checker %s failed: %v", name, err)
			return nil, err
		}
		checkers = append(checkers, checker)
	}

	return checkers, nil
}
