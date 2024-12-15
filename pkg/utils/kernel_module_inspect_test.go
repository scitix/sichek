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
