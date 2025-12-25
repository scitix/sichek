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

package specgen

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/oss"
	"github.com/sirupsen/logrus"
)

func promptString(msg string, def ...string) string {
	reader := bufio.NewReader(os.Stdin)
	if len(def) > 0 && def[0] != "" {
		fmt.Printf("%s [%s]: ", msg, def[0])
	} else {
		fmt.Printf("%s: ", msg)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" && len(def) > 0 {
		return def[0]
	}
	return input
}

func promptInt(msg string, def ...int) int {
	var valStr string
	if len(def) > 0 {
		valStr = promptString(msg, fmt.Sprintf("%d", def[0]))
	} else {
		valStr = promptString(msg)
	}
	if valStr == "" && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.Atoi(valStr)
	return val
}

func promptBool(msg string, def ...bool) bool {
	var valStr string
	if len(def) > 0 {
		valStr = promptString(msg, fmt.Sprintf("%t", def[0]))
	} else {
		valStr = promptString(msg)
	}
	if valStr == "y" || valStr == "Y" || valStr == "yes" || valStr == "Yes" || valStr == "YES" {
		return true
	}
	if valStr == "n" || valStr == "N" || valStr == "no" || valStr == "No" || valStr == "NO" {
		return false
	}
	if valStr == "" && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.ParseBool(valStr)
	return val
}

func promptFloat(msg string, def ...float64) float64 {
	var valStr string
	if len(def) > 0 {
		valStr = promptString(msg, fmt.Sprintf("%g", def[0]))
	} else {
		valStr = promptString(msg)
	}
	if valStr == "" && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.ParseFloat(valStr, 64)
	return val
}

// EnsureSpecFile ensures a spec file exists locally, downloading from OSS if needed
func EnsureSpecFile(specName string) (string, error) {
	// if specName is empty, return empty to caller to use default spec file
	if specName == "" {
		return "", nil
	}

	targetDir := consts.DefaultProductionCfgPath
	specPath := specName
	if !strings.Contains(specName, "/") {
		specPath = filepath.Join(targetDir, specName)
	}
	// Check if spec file already exists in target path
	if _, err := os.Stat(specPath); err == nil {
		logrus.WithField("component", "specgen").Infof("spec file found in target path: %s", specPath)
		return specPath, nil
	}

	// Download from OSS: specName maybe is a URL or a file name
	fileURL := specName
	if !strings.HasPrefix(fileURL, "http://") && !strings.HasPrefix(fileURL, "https://") {
		ossPath := oss.GetOssCfgPath()
		if ossPath == "" {
			return "", fmt.Errorf("OSS_URL environment variable is not set, cannot download spec from OSS")
		}
		fileURL = fmt.Sprintf("%s/%s", ossPath, specName)
	} else {
		parsedURL, err := url.Parse(fileURL)
		if err != nil {
			return "", fmt.Errorf("failed to parse URL: %v", err)
		}
		specName = path.Base(parsedURL.Path)
		specPath = filepath.Join(targetDir, specName)
	}
	logrus.WithField("component", "specgen").Infof("downloading spec file %s from OSS to %s", fileURL, specPath)
	err := oss.Download(fileURL, specPath)
	if err != nil {
		return "", fmt.Errorf("failed to download spec file %s from OSS: %v", fileURL, err)
	}
	return specPath, nil
}
