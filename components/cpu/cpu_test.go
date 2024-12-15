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
package cpu

import (
	"context"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/sirupsen/logrus"
)

func TestHealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	component, err := NewComponent("")
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("create cpu component failed: %v", err)
		return
	}

	result, err := component.HealthCheck(ctx)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("analyze cpu failed: %v", err)
		return
	}

	info, err := result.JSON()
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("result Marshal failed: %v", err)
	}
	logrus.WithField("component", "cpu").Infof("cpu analysis result: \n%s", common.ToString(info))
}
