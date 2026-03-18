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

// Package common provides shared utilities for the sichek component system.
// The spec.go file centralises the three spec-loading concerns:
//
//   - EnsureSpecFile: "where is the config?" – path discovery + OSS download + backup/tracing
//   - LoadSpec:       "how to read it?"        – pure YAML unmarshal, no side-effects
//   - FilterSpec:     "which one is mine?"     – in-memory map lookup + write-back with backup
package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// ─── LoadSpec ────────────────────────────────────────────────────────────────

// LoadSpec reads the YAML file at `file` and unmarshals it into `out`.
// This is a pure read operation: no network calls, no writes.
func LoadSpec[T any](file string, out *T) error {
	if file == "" {
		return fmt.Errorf("spec file path is empty")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read spec file %s: %w", file, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to unmarshal YAML from %s: %w", file, err)
	}
	return nil
}

// ─── EnsureSpecFile ──────────────────────────────────────────────────────────

// EnsureSpecFile guarantees that the canonical spec file (defaultFileName) is
// up-to-date and available on the local disk.
//
// The canonical file is always <DefaultProductionCfgPath>/<defaultFileName>
// (e.g. /var/sichek/config/default_spec.yaml).  All subsequent operations
// (EnsureSpec, LoadSpec, FilterSpec write-back) work exclusively on this file.
//
// Resolution order for the source content:
//  1. specName empty   → derive cluster name (e.g. "changliu_spec.yaml") and
//                        download it from SICHEK_SPEC_URL into the canonical file
//  2. specName is URL  → download directly into the canonical file
//  3. specName is existing path → copy into the canonical file
//  4. specName is bare filename → check default dir first, then try SICHEK_SPEC_URL
//  5. Fall back: use existing canonical file if already present
//
// Every overwrite is preceded by a .bak backup and traced via logrus.
func EnsureSpecFile(specName, defaultFileName string) (string, error) {
	const comp = "common/spec"
	targetDir := defaultProductionCfgPath()

	// The canonical destination is ALWAYS defaultFileName
	destPath := filepath.Join(targetDir, defaultFileName)

	// Resolve the source spec name
	sourceName := specName
	if sourceName == "" {
		cluster := extractClusterName()
		suffix := strings.TrimPrefix(defaultFileName, "default_")
		sourceName = fmt.Sprintf("%s_%s", cluster, suffix)
		logrus.WithField("component", comp).Infof("derived spec name from cluster: %s", sourceName)
	}

	// Case A: URL → download into canonical file
	if isHTTP(sourceName) {
		if err := DownloadSpecFile(sourceName, destPath, comp); err != nil {
			logrus.WithField("component", comp).Warnf("URL download failed (%v); falling back to existing %s", err, destPath)
		} else {
			return destPath, nil
		}
	} else if fileExists(sourceName) && sourceName != destPath {
		// Case B: explicit existing path (different from dest) → copy into canonical
		if err := overwriteWithBackup(sourceName, destPath, comp); err != nil {
			logrus.WithField("component", comp).Warnf("copy failed (%v); falling back to existing %s", err, destPath)
		} else {
			return destPath, nil
		}
	} else {
		// Case C: bare filename → check adjacent cluster file, then try OSS
		clusterPath := filepath.Join(targetDir, filepath.Base(sourceName))
		if fileExists(clusterPath) && clusterPath != destPath {
			logrus.WithField("component", comp).Infof("copying existing cluster file %s → %s", clusterPath, destPath)
			if err := overwriteWithBackup(clusterPath, destPath, comp); err == nil {
				return destPath, nil
			}
		}
		ossBase := os.Getenv("SICHEK_SPEC_URL")
		if ossBase != "" {
			fileURL := strings.TrimRight(ossBase, "/") + "/" + filepath.Base(sourceName)
			if err := DownloadSpecFile(fileURL, destPath, comp); err == nil {
				return destPath, nil
			} else {
				logrus.WithField("component", comp).Warnf("OSS download failed (%v); trying existing default", err)
			}
		} else {
			logrus.WithField("component", comp).Warnf("SICHEK_SPEC_URL not set, trying existing default")
		}
	}

	// Case D: fall back to whatever is already in the canonical file
	if fileExists(destPath) {
		logrus.WithField("component", comp).Infof("using existing canonical spec: %s", destPath)
		return destPath, nil
	}

	return "", fmt.Errorf("no spec file available at %s (SICHEK_SPEC_URL=%q)", destPath, os.Getenv("SICHEK_SPEC_URL"))
}

// ─── FilterSpec ──────────────────────────────────────────────────────────────

// FilterSpec loads the multi-spec container at `file`, selects the entry for
// `hardwareID` via `filterFn`, then **atomically rewrites** `file` with only
// that entry under `rootKey`.
//
// RootKey ensures we only update the component's section in a potentially
// shared YAML file (e.g. default_spec.yaml).
func FilterSpec[T any, S any](
	file string,
	rootKey string,
	hardwareID string,
	filterFn func(*T, string) (S, bool),
) (S, error) {
	var zero S
	const comp = "common/spec"

	if file == "" {
		return zero, fmt.Errorf("FilterSpec: file path is empty")
	}

	// Load container
	var container T
	if err := LoadSpec(file, &container); err != nil {
		return zero, fmt.Errorf("FilterSpec: %w", err)
	}

	// Select entry
	result, ok := filterFn(&container, hardwareID)
	if !ok {
		return zero, fmt.Errorf("FilterSpec: no entry for hardwareID=%q in %s[%s]",
			hardwareID, file, rootKey)
	}

	// Persist the filtered entry under rootKey (preserving map structure so sub-runs still find it)
	updatedSection := map[string]S{hardwareID: result}
	if err := surgicalUpdateYAML(file, rootKey, updatedSection, comp); err != nil {
		return zero, fmt.Errorf("FilterSpec: %w", err)
	}

	logrus.WithField("component", comp).Infof(
		"filtered & wrote %s[%s] (id=%s)", file, rootKey, hardwareID)
	return result, nil
}

// ─── WriteSpec ───────────────────────────────────────────────────────────────

// WriteSpec marshals `spec` to YAML and atomically overwrites only the
// `rootKey` section of `file`.
func WriteSpec[T any](file, rootKey, logComp string, spec *T) error {
	if file == "" {
		return fmt.Errorf("WriteSpec: file path is empty")
	}

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return fmt.Errorf("WriteSpec: mkdir failed: %w", err)
	}

	if err := surgicalUpdateYAML(file, rootKey, spec, logComp); err != nil {
		return fmt.Errorf("WriteSpec: %w", err)
	}

	logrus.WithField("component", logComp).Infof(
		"wrote %s[%s] baseline (ts=%s)", file, rootKey, time.Now().Format(time.RFC3339))
	return nil
}

// ─── MergeAndWriteSpec ───────────────────────────────────────────────────────

// MergeAndWriteSpec merges `entries` into the `rootKey` section of `file`
// (existing entries for sub-keys are NOT overwritten) and writes the result
// back surgically.
func MergeAndWriteSpec[T any, V any](
	file string,
	rootKey string,
	entries map[string]*V,
	getMap func(*T) map[string]*V,
	setMap func(*T, map[string]*V),
) error {
	const comp = "common/spec"

	// Load existing container (may be empty)
	var container T
	_ = LoadSpec(file, &container)

	m := getMap(&container)
	if m == nil {
		m = make(map[string]*V)
	}
	added := 0
	for k, v := range entries {
		if _, exists := m[k]; !exists {
			m[k] = v
			added++
		}
	}
	if added == 0 {
		logrus.WithField("component", comp).Infof("no new entries to merge into %s[%s]", file, rootKey)
		return nil
	}
	setMap(&container, m)

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return fmt.Errorf("MergeAndWriteSpec: mkdir failed: %w", err)
	}

	if err := surgicalUpdateYAML(file, rootKey, container, comp); err != nil {
		return fmt.Errorf("MergeAndWriteSpec: %w", err)
	}

	logrus.WithField("component", comp).Infof(
		"merged %d new entries into %s[%s] (ts=%s)", added, file, rootKey, time.Now().Format(time.RFC3339))
	return nil
}

// ─── surgicalUpdateYAML ──────────────────────────────────────────────────────

func surgicalUpdateYAML(file, rootKey string, sectionData interface{}, logComp string) error {
	// 1. Read existing file into map[string]interface{} (preserving unrelated keys)
	allData := make(map[string]interface{})
	if fileExists(file) {
		raw, err := os.ReadFile(file)
		if err == nil {
			_ = yaml.Unmarshal(raw, &allData)
		}
	}

	// 2. Patch the specific component's key
	// If sectionData is themselves a container struct (like NvidiaSpecs), we need to extract the component key's value.
	// But to keep it simple and robust, we expect sectionData to be the "content" of the rootKey.
	// However, components are passing T (the container).
	// Let's use reflection or just assume if it's a struct with that key, we unwrap it?
	// Actually, NvidiaSpecs has a field tagged `yaml:"nvidia"`.
	// If I put `container` directly into `allData["nvidia"]`, I'll get `nvidia: { nvidia: { ... } }`.
	// So call-sites should pass the map or we unwrap it here.

	// Let's unwrap if it's the same key to avoid nesting
	dataBytes, _ := yaml.Marshal(sectionData)

	var unwrapped map[string]interface{}
	if err := yaml.Unmarshal(dataBytes, &unwrapped); err != nil {
		logrus.WithField("component", logComp).Warnf("failed to unwrap section data: %v", err)
		allData[rootKey] = sectionData
	} else {
		if val, ok := unwrapped[rootKey]; ok {
			allData[rootKey] = val
		} else {
			allData[rootKey] = unwrapped
		}
	}

	// Double check: if allData[rootKey] is empty but sectionData is not, we might have lost data
	if allData[rootKey] == nil && sectionData != nil {
		allData[rootKey] = sectionData
	}

	// 3. Backup and atomic write
	bakPath := file + ".bak"
	if fileExists(file) {
		_ = copyFile(file, bakPath)
	}

	finalData, err := yaml.Marshal(allData)
	if err != nil {
		return fmt.Errorf("marshal all: %w", err)
	}

	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, finalData, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, file); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ─── DownloadSpecFile ────────────────────────────────────────────────────────

// DownloadSpecFile downloads a single spec YAML from `fileURL` and saves it to
// `destPath`, backing up any pre-existing file first.
// Exposed for use by component.EnsureSpec implementations.
func DownloadSpecFile(fileURL, destPath, logComp string) error {
	if fileExists(destPath) {
		bak := destPath + ".bak"
		if err := copyFile(destPath, bak); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		logrus.WithField("component", logComp).Infof("backed up %s → %s", destPath, bak)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	tmp := destPath + ".tmp"
	if err := httpDownload(fileURL, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename failed: %w", err)
	}
	logrus.WithField("component", logComp).Infof(
		"downloaded %s → %s (ts=%s)", fileURL, destPath, time.Now().Format(time.RFC3339))
	return nil
}

// ─── internal helpers ────────────────────────────────────────────────────────

func defaultProductionCfgPath() string {
	if p := os.Getenv("SICHEK_CONFIG_DIR"); p != "" {
		return p
	}
	return "/var/sichek/config"
}

func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func overwriteWithBackup(src, dst, logComp string) error {
	if !fileExists(src) {
		return fmt.Errorf("source file %s does not exist", src)
	}

	// Backup destination if it exists
	if fileExists(dst) {
		bak := dst + ".bak"
		if err := copyFile(dst, bak); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		logrus.WithField("component", logComp).Infof("backed up %s → %s", dst, bak)
	}

	// Copy src to dst
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	logrus.WithField("component", logComp).Infof("overwrote %s with %s", dst, src)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func httpDownload(fileURL, destPath string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fileURL)
	if err != nil {
		return fmt.Errorf("GET %s: %w", fileURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", fileURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body from %s: %w", fileURL, err)
	}
	return os.WriteFile(destPath, data, 0644)
}

func extractClusterName() string {
	name := os.Getenv("NODE_NAME")
	if name == "" {
		if h, err := os.Hostname(); err == nil {
			name = h
		}
	}
	if name == "" {
		return "default"
	}
	end := 0
	for end < len(name) {
		c := name[end]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			end++
		} else {
			break
		}
	}
	if end == 0 {
		return "default"
	}
	return name[:end]
}
