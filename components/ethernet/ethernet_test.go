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
package ethernet

import (
	"context"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
)

func TestHealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	component, err := NewEthernetComponent("")
	if err != nil {
		t.Fatalf("failed to create Ethernet component: %v", err)
	}
	if err != nil {
		t.Fatalf("failed to create Ethernet component: %v", err)
	}
	result, err := component.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("failed to Ethernet HealthCheck: %v", err)
		return
	}
	output := common.ToString(result)
	t.Logf("Ethernet Analysis Result: \n%s", output)
}
