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
	if configName == "" {
		return filepath.Join(consts.DefaultProductionCfgPath, consts.DefaultUserCfgName), nil
	}
	return EnsureSpecFile(configName)
}

// EnsureSpecFile ensures a spec file is available locally.
// It follows this priority order:
//  1. Empty specName -> use cluster name to get spec file name
//  2. URL           -> download to default spec dir
//  3. Existing path -> use directly
//  4. Filename      -> check default dir, otherwise download from SICHEK_SPEC_URL
func EnsureSpecFile(specName string) (string, error) {
	// If specName is empty, use cluster name to get spec file name
	if specName == "" {
		clusterName := utils.ExtractClusterName()
		specName = fmt.Sprintf("%s_spec.yaml", clusterName)
	}

	targetDir := consts.DefaultProductionCfgPath

	// Case 1: URL - download to default spec dir
	if isURL(specName) {
		return downloadSpec(specName, targetDir)
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
		return "", fmt.Errorf("spec file %q not found locally and SICHEK_SPEC_URL is not set", specName)
	}

	fileURL := fmt.Sprintf("%s/%s", strings.TrimRight(specURL, "/"), fileName)
	return downloadSpecToPath(fileURL, specPath)
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
