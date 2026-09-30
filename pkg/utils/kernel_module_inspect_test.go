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
package utils

import (
	"testing"
)

func TestIsKernalModuleLoaded(t *testing.T) {
	module := "mlx5_core"

	// Test when the module is not loaded
	loaded, err := IsKernalModuleLoaded(module)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !loaded {
		t.Errorf("Expected loaded, got unload")
	}
}

func TestIsKernalModuleHolder(t *testing.T) {
	holder := "mlx5_core"
	module := "mlx5_ib"

	// Test when the holder module does not exist
	exists, err := IsKernalModuleHolder(holder, module)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("Expected true, got false")
	}
}
