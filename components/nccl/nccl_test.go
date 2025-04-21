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
package nccl

import (
	"context"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
)

func TestNCCL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	component, err := NewComponent("", "")
	if err != nil {
		t.Log(err)
		return
	}

	result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)
	if err != nil {
		t.Log(err)
	}
	js, errr := result.JSON()
	if errr != nil {
		t.Log(errr)
	}
	t.Logf("test nccl analysis result: %s", js)
	t.Logf("Running time: %ds", time.Since(start))
}
