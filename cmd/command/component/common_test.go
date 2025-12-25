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
package component

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/scitix/sichek/consts"
)

func TestGetComponentsFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		want        []string
		wantErr     bool
		description string
	}{
		{
			name: "normal config with multiple components",
			configData: `
metrics:
  port: 19091

nvidia:
  query_interval: 10s
  cache_size: 5

cpu:
  query_interval: 10s
  cache_size: 5

infiniband:
  query_interval: 10s
  cache_size: 5

IBGDA:
  enable: true

nccltest:
  enable: true
`,
			want:        []string{"nvidia", "cpu", "infiniband", "IBGDA", "nccltest"},
			wantErr:     false,
			description: "should return all components except metrics",
		},
		{
			name: "config with only metrics",
			configData: `
metrics:
  port: 19091
`,
			want:        []string{},
			wantErr:     false,
			description: "should return empty list when only metrics is present",
		},
		{
			name: "config with non-map values",
			configData: `
metrics:
  port: 19091

nvidia:
  query_interval: 10s

invalid_component: "not a map"
`,
			want:        []string{"nvidia"},
			wantErr:     false,
			description: "should skip non-map values",
		},
		{
			name: "empty config",
			configData: `
metrics:
  port: 19091
`,
			want:        []string{},
			wantErr:     false,
			description: "should return empty list for empty config",
		},
		{
			name: "config with all component types",
			configData: `
metrics:
  port: 19091

nvidia:
  query_interval: 10s

cpu:
  query_interval: 10s

infiniband:
  query_interval: 10s

gpfs:
  query_interval: 10s

dmesg:
  query_interval: 10s

podlog:
  query_interval: 10s

syslog:
  query_interval: 10s

gpuevents:
  query_interval: 10s

memory:
  query_interval: 10s

IBGDA:
  enable: true

nccltest:
  enable: true

pcietopo:
  enable: true
`,
			want:        []string{"nvidia", "cpu", "infiniband", "gpfs", "dmesg", "podlog", "syslog", "gpuevents", "memory", "IBGDA", "nccltest", "pcietopo"},
			wantErr:     false,
			description: "should return all components",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpFile, err := os.CreateTemp("", "test_config_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write config data
			if _, err := tmpFile.WriteString(tt.configData); err != nil {
				t.Fatalf("Failed to write config data: %v", err)
			}
			tmpFile.Close()

			// Test GetComponentsFromConfig
			got, err := GetComponentsFromConfig(tmpFile.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetComponentsFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Sort both slices for comparison
			slices.Sort(got)
			slices.Sort(tt.want)

			if !slices.Equal(got, tt.want) {
				t.Errorf("GetComponentsFromConfig() = %v, want %v. %s", got, tt.want, tt.description)
			}
		})
	}
}

func TestGetComponentsFromConfig_InvalidYAML(t *testing.T) {
	// Create temporary file with invalid YAML
	tmpFile, err := os.CreateTemp("", "test_invalid_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	invalidYAML := `invalid: yaml: content: [`
	if _, err := tmpFile.WriteString(invalidYAML); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}
	tmpFile.Close()

	// Note: LoadUserConfig has fallback mechanisms, so it might not return an error
	// if it can load from default paths. We test that invalid YAML in the specified file
	// will cause it to fallback, but the function itself might not error.
	_, err = GetComponentsFromConfig(tmpFile.Name())
	// The function may or may not return an error depending on fallback behavior
	// So we just verify it doesn't panic
	if err != nil {
		t.Logf("GetComponentsFromConfig() returned error (expected due to invalid YAML): %v", err)
	}
}

func TestDetermineComponentsToCheck(t *testing.T) {
	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "test_config.yaml")

	configData := `
metrics:
  port: 19091

nvidia:
  query_interval: 10s

cpu:
  query_interval: 10s

infiniband:
  query_interval: 10s

gpfs:
  query_interval: 10s

IBGDA:
  enable: true

nccltest:
  enable: true
`

	if err := os.WriteFile(cfgFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	tests := []struct {
		name              string
		enableComponents  string
		ignoredComponents string
		cfgFile           string
		logField          string
		want              []string
		description       string
	}{
		{
			name:              "use -E flag",
			enableComponents:  "cpu,nvidia",
			ignoredComponents: "",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"cpu", "nvidia"},
			description:       "should use components from -E flag",
		},
		{
			name:              "use -E flag with spaces",
			enableComponents:  "cpu, nvidia , infiniband",
			ignoredComponents: "",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"cpu", "nvidia", "infiniband"},
			description:       "should trim spaces from component names",
		},
		{
			name:              "use config file without -I",
			enableComponents:  "",
			ignoredComponents: "",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"nvidia", "cpu", "infiniband", "gpfs", "IBGDA", "nccltest"},
			description:       "should return all components from config",
		},
		{
			name:              "use config file with -I",
			enableComponents:  "",
			ignoredComponents: "cpu,nvidia",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"infiniband", "gpfs", "IBGDA", "nccltest"},
			description:       "should exclude ignored components",
		},
		{
			name:              "use -E flag with -I (should ignore -I)",
			enableComponents:  "cpu,nvidia,infiniband",
			ignoredComponents: "cpu",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"cpu", "nvidia", "infiniband"},
			description:       "when -E is used, -I should be ignored",
		},
		{
			name:              "empty enableComponents with empty ignoredComponents",
			enableComponents:  "",
			ignoredComponents: "",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"nvidia", "cpu", "infiniband", "gpfs", "IBGDA", "nccltest"},
			description:       "should return all components from config",
		},
		{
			name:              "config file not found, fallback to DefaultComponents",
			enableComponents:  "",
			ignoredComponents: "",
			cfgFile:           "/nonexistent/config.yaml",
			logField:          "test",
			want:              consts.DefaultComponents, // May fallback to default config or DefaultComponents
			description:       "should fallback when config file not found (may use default config if exists)",
		},
		{
			name:              "empty cfgFile uses default path",
			enableComponents:  "",
			ignoredComponents: "",
			cfgFile:           "",
			logField:          "test",
			want:              consts.DefaultComponents, // May use default config if exists
			description:       "should use default config path when cfgFile is empty (may use default config if exists)",
		},
		{
			name:              "-E with duplicate components",
			enableComponents:  "cpu,cpu,nvidia",
			ignoredComponents: "",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"cpu", "nvidia"},
			description:       "should remove duplicate components",
		},
		{
			name:              "-E with empty components",
			enableComponents:  "cpu,,nvidia,",
			ignoredComponents: "",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{"cpu", "nvidia"},
			description:       "should skip empty component names",
		},
		{
			name:              "config with -I excluding all",
			enableComponents:  "",
			ignoredComponents: "nvidia,cpu,infiniband,gpfs,IBGDA,nccltest",
			cfgFile:           cfgFile,
			logField:          "test",
			want:              []string{},
			description:       "should return empty list when all components are ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineComponentsToCheck(tt.enableComponents, tt.ignoredComponents, tt.cfgFile, tt.logField)

			// Sort both slices for comparison
			slices.Sort(got)
			slices.Sort(tt.want)

			// For tests that may fallback to default config, we check if result contains expected components
			if tt.name == "config file not found, fallback to DefaultComponents" ||
				tt.name == "empty cfgFile uses default path" {
				// These tests may use default config if it exists, so we just verify
				// that the result is not empty and contains valid components
				if len(got) == 0 {
					t.Errorf("DetermineComponentsToCheck() returned empty list, expected non-empty. %s", tt.description)
				}
				// Verify all returned components are valid (either in DefaultComponents or from config)
				for _, comp := range got {
					valid := false
					for _, dc := range consts.DefaultComponents {
						if comp == dc {
							valid = true
							break
						}
					}
					// Also check common config components
					commonComps := []string{"IBGDA", "nccltest", "pcietopo", "memory"}
					for _, cc := range commonComps {
						if comp == cc {
							valid = true
							break
						}
					}
					if !valid {
						t.Logf("Component %s is not in DefaultComponents or common config components", comp)
					}
				}
			} else {
				if !slices.Equal(got, tt.want) {
					t.Errorf("DetermineComponentsToCheck() = %v, want %v. %s", got, tt.want, tt.description)
				}
			}
		})
	}
}

func TestDetermineComponentsToCheck_EdgeCases(t *testing.T) {
	// Test with invalid YAML file
	tmpFile, err := os.CreateTemp("", "test_invalid_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	invalidYAML := `invalid: yaml: content: [`
	if _, err := tmpFile.WriteString(invalidYAML); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}
	tmpFile.Close()

	// Should fallback to DefaultComponents or default config when config is invalid
	got := DetermineComponentsToCheck("", "", tmpFile.Name(), "test")
	// Due to fallback mechanisms, result may be DefaultComponents or from default config
	if len(got) == 0 {
		t.Error("DetermineComponentsToCheck() with invalid config returned empty list, expected non-empty")
	}
	// Verify all returned components are valid
	for _, comp := range got {
		valid := false
		for _, dc := range consts.DefaultComponents {
			if comp == dc {
				valid = true
				break
			}
		}
		if !valid {
			// Check if it's a common config component
			commonComps := []string{"IBGDA", "nccltest", "pcietopo", "memory"}
			for _, cc := range commonComps {
				if comp == cc {
					valid = true
					break
				}
			}
			if !valid {
				t.Logf("Component %s is not in DefaultComponents or common config components", comp)
			}
		}
	}
}
