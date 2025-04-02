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
package hang

import (
	"context"
	"testing"
	"time"

	"github.com/scitix/sichek/components/nvidia"
)

func TestHang(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	start := time.Now()
	_, err := nvidia.NewComponent("", "", nil)
	if err != nil {
		t.Fatalf("Failed too New Nvidia component, Bypassing HealthCheck")
	}
	component, err := NewComponent("./config/config_mock_test.yaml")
	if err != nil {
		t.Log(err)
		return
	}

	for i := 0; i < 100; i++ {
		_, err := component.HealthCheck(ctx)
		if err != nil {
			t.Error(err)
		}
	}
	result, err := component.HealthCheck(ctx)

	if err != nil {
		t.Log(err)
	}
	js, err := result.JSON()
	if err != nil {
		t.Log(err)
	}
	t.Logf("test hang analysis result: %s", js)
	t.Logf("Running time: %ds", time.Since(start))
}
