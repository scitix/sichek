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
package common

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// ─── shared test types ────────────────────────────────────────────────────────

type itemSpec struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type multiItemSpec struct {
	Items map[string]*itemSpec `json:"items" yaml:"items"`
}

type nvidiaSpec struct {
	Name string `json:"name" yaml:"name"`
}

type nvidiaSpecs struct {
	Specs map[string]*nvidiaSpec `json:"nvidia" yaml:"nvidia"`
}

// ─── LoadSpec ────────────────────────────────────────────────────────────────

func TestLoadSpec_OK(t *testing.T) {
	f := writeTmpYAML(t, itemSpec{Name: "hello", Version: "v1"})
	var out itemSpec
	if err := LoadSpec(f, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "hello" || out.Version != "v1" {
		t.Errorf("got %+v", out)
	}
}

func TestLoadSpec_EmptyPath(t *testing.T) {
	var out itemSpec
	if err := LoadSpec("", &out); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestLoadSpec_TypeMismatch(t *testing.T) {
	// "name" field is a mapping, but our struct expects a string → unmarshal error
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yaml")
	os.WriteFile(f, []byte("name:\n  nested: true\n"), 0644)
	type strSpec struct {
		Name string `yaml:"name"`
	}
	var out strSpec
	if err := LoadSpec(f, &out); err == nil {
		t.Error("expected error for type-mismatch YAML")
	}
}

// ─── EnsureSpecFile ──────────────────────────────────────────────────────────

func TestEnsureSpecFile_ExistingLocalPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SICHEK_CONFIG_DIR", dir)

	f := writeTmpYAML(t, itemSpec{Name: "local"})
	defaultFile := "default_spec.yaml"
	got, err := EnsureSpecFile(f, defaultFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(dir, defaultFile)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	// Verify content was copied
	var out itemSpec
	if err := LoadSpec(got, &out); err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if out.Name != "local" {
		t.Errorf("expected Name=local, got %s", out.Name)
	}
}

func TestEnsureSpecFile_DownloadAndBackup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "name: remote\nversion: v2")
	}))
	defer srv.Close()

	dir := t.TempDir()
	destPath := filepath.Join(dir, "some_spec.yaml")
	writeTmpYAMLTo(t, destPath, itemSpec{Name: "original"})

	// downloadAndBackup is unexported; test via DownloadSpecFile directly
	err := DownloadSpecFile(srv.URL+"/some_spec.yaml", destPath, "test")
	if err != nil {
		t.Fatalf("DownloadSpecFile: %v", err)
	}

	// Backup must exist and contain the original
	bak := destPath + ".bak"
	if !fileExists(bak) {
		t.Fatal("backup not created")
	}
	var bakSpec itemSpec
	if err := LoadSpec(bak, &bakSpec); err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if bakSpec.Name != "original" {
		t.Errorf("backup should contain original, got %+v", bakSpec)
	}
}

// ─── FilterSpec ──────────────────────────────────────────────────────────────

func filterItem(c *multiItemSpec, id string) (*itemSpec, bool) {
	v, ok := c.Items[id]
	return v, ok
}

func TestFilterSpec_OK(t *testing.T) {
	f := writeTmpYAML(t, multiItemSpec{Items: map[string]*itemSpec{
		"a1": {Name: "Alpha", Version: "v1"},
		"b2": {Name: "Beta", Version: "v2"},
	}})

	result, err := FilterSpec(f, "items", "a1", filterItem)
	if err != nil {
		t.Fatalf("FilterSpec: %v", err)
	}
	if result == nil || result.Name != "Alpha" {
		t.Errorf("unexpected result: %+v", result)
	}

	// Backup must exist
	if !fileExists(f + ".bak") {
		t.Error("backup not created")
	}
	// Overwritten file must NOT contain "Beta"
	raw, _ := os.ReadFile(f)
	if strings.Contains(string(raw), "Beta") {
		t.Error("overwritten file should not contain Beta")
	}
}

func TestFilterSpec_NotFound_Restores(t *testing.T) {
	f := writeTmpYAML(t, multiItemSpec{Items: map[string]*itemSpec{
		"a1": {Name: "Alpha"},
	}})
	original, _ := os.ReadFile(f)

	_, err := FilterSpec(f, "items", "nonexistent", filterItem)
	if err == nil {
		t.Fatal("expected error")
	}
	// File must be restored
	restored, _ := os.ReadFile(f)
	if string(restored) != string(original) {
		t.Error("file not restored after failure")
	}
}

func TestFilterSpec_EmptyPath(t *testing.T) {
	_, err := FilterSpec("", "items", "id", filterItem)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

// ─── MergeAndWriteSpec ───────────────────────────────────────────────────────

func TestMergeAndWriteSpec_AddsNewEntries(t *testing.T) {
	f := writeTmpYAML(t, multiItemSpec{Items: map[string]*itemSpec{
		"a1": {Name: "Alpha"},
	}})

	err := MergeAndWriteSpec(
		f,
		"items",
		map[string]*itemSpec{"b2": {Name: "Beta"}},
		func(c *multiItemSpec) map[string]*itemSpec { return c.Items },
		func(c *multiItemSpec, m map[string]*itemSpec) { c.Items = m },
	)
	if err != nil {
		t.Fatalf("MergeAndWriteSpec: %v", err)
	}

	var result multiItemSpec
	if err := LoadSpec(f, &result); err != nil {
		t.Fatalf("LoadSpec after merge: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result.Items))
	}
}

func TestMergeAndWriteSpec_DoesNotOverwrite(t *testing.T) {
	f := writeTmpYAML(t, multiItemSpec{Items: map[string]*itemSpec{
		"a1": {Name: "Alpha", Version: "v1"},
	}})
	MergeAndWriteSpec(
		f,
		"items",
		map[string]*itemSpec{"a1": {Name: "AlphaOverwrite", Version: "v99"}},
		func(c *multiItemSpec) map[string]*itemSpec { return c.Items },
		func(c *multiItemSpec, m map[string]*itemSpec) { c.Items = m },
	)
	var result multiItemSpec
	LoadSpec(f, &result)
	if result.Items["a1"].Name != "Alpha" {
		t.Error("existing entry should not be overwritten")
	}
}

func TestFilterSpec_SurgicalPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "shared.yaml")

	// 1. Write a file with two sections
	initial := `
nvidia:
  "0x1": {name: "N1"}
  "0x2": {name: "N2"}
infiniband:
  changliu: {ib_devs: {mlx5_0: eth0}}
`
	os.WriteFile(f, []byte(initial), 0644)

	// 2. Filter nvidia section
	_, err := FilterSpec(f, "nvidia", "0x1", func(c *nvidiaSpecs, id string) (*nvidiaSpec, bool) {
		s, ok := c.Specs[id]
		return s, ok
	})
	if err != nil {
		t.Fatalf("FilterSpec: %v", err)
	}

	// 3. Verify that nvidia is filtered but infiniband is preserved
	raw, _ := os.ReadFile(f)
	content := string(raw)

	if !strings.Contains(content, "infiniband:") {
		t.Error("lost infiniband section!")
	}
	if !strings.Contains(content, "changliu:") {
		t.Error("lost changliu inside infiniband!")
	}
	if strings.Contains(content, "0x2") {
		t.Error("failed to filter out 0x2 from nvidia!")
	}
	if !strings.Contains(content, "0x1") {
		t.Error("lost 0x1 from nvidia!")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeTmpYAML(t *testing.T, v interface{}) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "spec.yaml")
	writeTmpYAMLTo(t, f, v)
	return f
}

func writeTmpYAMLTo(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
