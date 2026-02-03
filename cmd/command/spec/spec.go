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
package spec

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

// EnsureCfgFile ensures a config file is available locally.
// If configName is empty, returns the default config file path.
// Otherwise, delegates to EnsureSpecFile.
func EnsureCfgFile(configName string) (string, error) {
	return ensureSpecFile(configName, consts.DefaultUserCfgName)
}

func EnsureSpecFile(specName string) (string, error) {
	return ensureSpecFile(specName, consts.DefaultSpecCfgName)
}

// ensureSpecFile ensures a spec/config file is available locally.
// defaultFileName is e.g. default_spec.yaml or default_user_config.yaml;
// when specName is empty, the cluster-specific filename is clusterName + "_" + suffix from defaultFileName (e.g. xxx_spec.yaml or xxx_user_config.yaml).
// It follows this priority order:
//  1. Empty specName -> use cluster name + suffix from defaultFileName (e.g. xxx_user_config.yaml or xxx_spec.yaml)
//  2. URL           -> download to default dir
//  3. Existing path -> use directly
//  4. Filename      -> check default dir, otherwise download from SICHEK_SPEC_URL; if not found, fall back to defaultFileName
func ensureSpecFile(specName string, defaultFileName string) (string, error) {
	// If specName is empty, use cluster name + suffix from default (e.g. default_user_config.yaml -> taihua_user_config.yaml)
	if specName == "" {
		clusterName := utils.ExtractClusterName()
		suffix := strings.TrimPrefix(defaultFileName, "default_")
		specName = fmt.Sprintf("%s_%s", clusterName, suffix)
	}

	targetDir := consts.DefaultProductionCfgPath

	// Case 1: URL - download to default spec dir
	if isURL(specName) {
		path, err := downloadSpec(specName, targetDir)
		if err != nil {
			logrus.WithField("component", "specgen").Warnf("download from URL failed: %v, falling back to %s", err, defaultFileName)
			return fallbackToDefaultSpec(defaultFileName)
		}
		return path, nil
	}

	// Case 2: Existing local path (absolute or relative) - use directly
	if fileExists(specName) {
		logrus.WithField("component", "specgen").Warnf("using existing spec file at path: %s", specName)
		return specName, nil
	}

	// Case 3: Treat as filename under default directory
	fileName := filepath.Base(specName)
	specPath := filepath.Join(targetDir, fileName)

	if fileExists(specPath) {
		logrus.WithField("component", "specgen").Warnf("using existing spec file in default dir: %s", specPath)
		return specPath, nil
	}

	// Case 4: Download from SICHEK_SPEC_URL
	specURL := httpclient.GetSichekSpecURL()
	if specURL == "" {
		logrus.WithField("component", "specgen").Warnf("SICHEK_SPEC_URL is not set, falling back to %s", defaultFileName)
		return fallbackToDefaultSpec(defaultFileName)
	}

	fileURL := fmt.Sprintf("%s/%s", strings.TrimRight(specURL, "/"), fileName)
	path, err := downloadSpecToPath(fileURL, specPath)
	if err != nil {
		logrus.WithField("component", "specgen").Warnf("download from URL %s failed: %v, falling back to %s", fileURL, err, defaultFileName)
		return fallbackToDefaultSpec(defaultFileName)
	}
	return path, nil
}

func fallbackToDefaultSpec(defaultFileName string) (string, error) {
	defaultSpecPath := filepath.Join(consts.DefaultProductionCfgPath, defaultFileName)
	if fileExists(defaultSpecPath) {
		logrus.WithField("component", "specgen").Warnf("using fallback default file: %s", defaultSpecPath)
		return defaultSpecPath, nil
	}
	return "", fmt.Errorf("default %s file does not exist", defaultFileName)
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func downloadSpec(URL, targetDir string) (string, error) {
	parsed, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("invalid spec URL %q: %w", URL, err)
	}

	fileName := path.Base(parsed.Path)
	specPath := filepath.Join(targetDir, fileName)

	return downloadSpecToPath(URL, specPath)
}

func downloadSpecToPath(url, specPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create spec dir: %w", err)
	}

	tmpPath := specPath + ".tmp"

	logrus.WithFields(logrus.Fields{
		"component": "specgen",
		"url":       url,
		"path":      specPath,
	}).Info("downloading spec file")

	if err := httpclient.Download(url, tmpPath); err != nil {
		return "", fmt.Errorf("failed to download spec from %s: %w", url, err)
	}

	if err := os.Rename(tmpPath, specPath); err != nil {
		return "", fmt.Errorf("failed to move spec file into place: %w", err)
	}

	logrus.WithField("component", "specgen").Warnf("using spec file %s, downloaded from %s", specPath, url)
	return specPath, nil
}
