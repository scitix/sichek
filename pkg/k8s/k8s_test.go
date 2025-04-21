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
package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUpdateNodeAnnotation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	annotation := map[string]string{
		"scitix.ai/sichek": "",
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create k8s client: %v", err)
	}
	err = client.UpdateNodeAnnotation(ctx, annotation)
	if err != nil {
		t.Errorf("failed to update node annotation to %v: %v", annotation, err)
	}
	node, err := client.GetCurrNode(ctx)
	if err != nil {
		t.Errorf("failed to get node: %v", err)
	}
	fmt.Printf("new annotation: %s", node.Annotations["scitix.ai/sichek"])
}
