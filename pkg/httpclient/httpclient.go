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
package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/scitix/sichek/consts"
)

var (
	specURL     string
	specURLOnce sync.Once
)

func GetSichekSpecURL() string {
	specURLOnce.Do(func() {
		// 1. 尝试从环境变量获取
		if envURL := os.Getenv("SICHEK_SPEC_URL"); envURL != "" {
			specURL = envURL
			return
		}

		// 2. 尝试从本地配置文件获取
		configPath := filepath.Join(consts.DefaultProductionCfgPath, consts.DefaultUserCfgName)
		if urlFromConfig := getSpecURLFromConfig(configPath); urlFromConfig != "" {
			specURL = urlFromConfig
			return
		}

		// 3. 自动探测并持久化
		specURL = probeAndSaveSpecURL(configPath)
	})
	return specURL
}

func getSpecURLFromConfig(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return ""
	}

	if val, ok := config["sichek_spec_url"].(string); ok {
		return val
	}
	return ""
}

func probeAndSaveSpecURL(configPath string) string {
	probeTargets := []string{consts.DomesticSpecURL, consts.OverseasSpecURL}
	selectedURL := consts.DomesticSpecURL // 默认回归到国内

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	for _, baseURL := range probeTargets {
		testURL := fmt.Sprintf("%s/text.txt", baseURL)
		resp, err := client.Get(testURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				selectedURL = baseURL
				logrus.Infof("successfully probed spec URL: %s", selectedURL)
				break
			}
		}
	}

	// 持久化到配置文件
	saveSpecURLToConfig(configPath, selectedURL)
	return selectedURL
}

func saveSpecURLToConfig(configPath string, url string) {
	config := make(map[string]interface{})
	data, err := os.ReadFile(configPath)
	if err == nil {
		yaml.Unmarshal(data, &config)
	}

	config["sichek_spec_url"] = url
	newData, err := yaml.Marshal(config)
	if err != nil {
		logrus.Errorf("failed to marshal config for spec URL persistence: %v", err)
		return
	}

	// 确保目录存在
	os.MkdirAll(filepath.Dir(configPath), 0755)
	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		logrus.Errorf("failed to save spec URL to %s: %v", configPath, err)
	} else {
		logrus.Infof("persisted spec URL %s to %s", url, configPath)
	}
}

func HasSichekSpecURL() bool {
	return GetSichekSpecURL() != ""
}

// getDefaultClient returns a default HTTP client
func getDefaultClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
	}
}

// getConnectivityClient returns a client for connectivity checks
func getConnectivityClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second, // Short timeout for connectivity check
	}
}

// CheckConnectivity checks if the HTTP endpoint is reachable
func CheckConnectivity(url string) error {
	client := getConnectivityClient()
	if url == "" {
		url = GetSichekSpecURL()
		if url == "" {
			return fmt.Errorf("SICHEK_SPEC_URL environment variable is not set")
		}
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP endpoint %s is not reachable: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil // HTTP endpoint is reachable
	}
	return fmt.Errorf("HTTP endpoint %s returned status %d", url, resp.StatusCode)
}

// Download downloads a file from HTTP URL to local path
func Download(fileURL, targetPath string) error {
	if fileURL == "" {
		return fmt.Errorf("file name or url cannot be empty")
	}

	if targetPath == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	// Check connectivity first
	if err := CheckConnectivity(fileURL); err != nil {
		return fmt.Errorf("network connectivity check for %s failed: %v", fileURL, err)
	}

	// Download the file
	client := getDefaultClient()
	resp, err := client.Get(fileURL)
	if err != nil {
		return fmt.Errorf("failed to fetch file from %s: %v", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("file not found at %s (status: %d)", fileURL, resp.StatusCode)
	}

	// Create target directory if it doesn't exist
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Download and save the file
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read file content: %v", err)
	}

	var tmp interface{}
	if err := yaml.Unmarshal(body, &tmp); err != nil {
		return fmt.Errorf("file is not valid YAML: %v", err)
	}

	if err := os.WriteFile(targetPath, body, 0644); err != nil {
		return fmt.Errorf("failed to write file to %s: %v", targetPath, err)
	}

	logrus.WithField("component", "httpclient").Infof("successfully downloaded file to: %s", targetPath)
	return nil
}

// LoadSpecFromURL loads a spec from a given URL into the provided structure
func LoadSpecFromURL(url string, spec interface{}) error {
	if url == "" {
		return fmt.Errorf("URL is empty")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("unsupported URL scheme (must start with http:// or https://): %s", url)
	}

	// Check connectivity first
	if err := CheckConnectivity(url); err != nil {
		return fmt.Errorf("network connectivity check for %s failed: %v", url, err)
	}

	client := getDefaultClient()
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch spec from %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d while fetching %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body from %s: %v", url, err)
	}

	// Try to unmarshal as YAML
	if err := yaml.Unmarshal(data, spec); err != nil {
		return fmt.Errorf("failed to unmarshal YAML from %s: %v", url, err)
	}

	return nil
}

// Upload uploads a YAML spec file (as []byte) to HTTP-compatible storage.
func Upload(fileURL string, data []byte) error {
	if fileURL == "" {
		return fmt.Errorf("file url cannot be empty")
	}
	if len(data) == 0 {
		return fmt.Errorf("data cannot be empty")
	}

	// Check connectivity
	if err := CheckConnectivity(fileURL); err != nil {
		return fmt.Errorf("network connectivity check failed: %v", err)
	}

	client := getDefaultClient()

	// HTTP PUT request for file upload
	req, err := http.NewRequest("PUT", fileURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-yaml")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file to %s: %v", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload file (status %d): %s", resp.StatusCode, string(body))
	}

	logrus.Infof("✅ Successfully uploaded YAML file to: %s", fileURL)
	return nil
}

// CheckFileExists checks if a file exists at the given HTTP URL
func CheckFileExists(fileURL string) (bool, error) {
	if fileURL == "" {
		return false, fmt.Errorf("file name or url cannot be empty")
	}

	// Try to download to verify existence (without saving)
	if !strings.HasPrefix(fileURL, "http://") && !strings.HasPrefix(fileURL, "https://") {
		specURL := GetSichekSpecURL()
		if specURL == "" {
			return false, fmt.Errorf("SICHEK_SPEC_URL environment variable is not set")
		}
		fileURL = fmt.Sprintf("%s/%s", specURL, fileURL)
	}

	// Check HTTP connectivity first
	if err := CheckConnectivity(fileURL); err != nil {
		return false, fmt.Errorf("HTTP endpoint %s not reachable: %v", fileURL, err)
	}

	client := getDefaultClient()
	resp, err := client.Get(fileURL)
	if err != nil {
		return false, fmt.Errorf("failed to check file at %s: %v", fileURL, err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}
