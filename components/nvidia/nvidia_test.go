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
package nvidia

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/sirupsen/logrus"
)

func TestHealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	component, err := NewComponent("", nil)
	if err != nil {
		t.Fatalf("failed to create component: %v", err)
	}
	if err != nil {
		logrus.WithField("components", "Nvidia").Error("fail to Create Nvidia Components")
	}
	result, err := component.HealthCheck(ctx)
	if err != nil {
		logrus.WithField("component", component.Name()).Error(err)
		return
	}
	output := common.ToString(result)
	logrus.WithField("component", component.Name()).Infof("Analysis Result: \n%s", output)
}

func TestSplit(t *testing.T) {
	input := ":gpu"
	result := strings.Split(input, ":")
	if len(result) != 2 {
		t.Fatalf("failed to split string")
	}
	t.Logf("the first: %s, the second: %s", result[0], result[1])

	input = "device:"
	result = strings.Split(input, ":")
	if len(result) != 2 {
		t.Fatalf("failed to split string")
	}
	t.Logf("the first: %s, the second: %s", result[0], result[1])

	input = "device:gpu"
	result = strings.Split(input, ":")
	if len(result) != 2 {
		t.Fatalf("failed to split string")
	}
	t.Logf("the first: %s, the second: %s", result[0], result[1])
}