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
package syslog

import (
	"context"
	"fmt"
	"os"

	"testing"
	"time"

	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyslogComponentWithDefaultConfig(t *testing.T) {
	// Test component creation
	component, err := NewComponent("", "", 0)
	require.NoError(t, err)
	require.NotNil(t, component)

	// Test component name
	assert.Equal(t, consts.ComponentNameSyslog, component.Name())

	// Test timeout
	timeout := component.GetTimeout()
	assert.Equal(t, 30*time.Second, timeout)
}

func TestNewSyslogComponent(t *testing.T) {
	// Create temporary config file
	configFile, err := os.CreateTemp("", "syslog_cfg_*.yaml")
	require.NoError(t, err)
	defer os.Remove(configFile.Name())

	configData := `
syslog:
  query_interval: 60s
  cache_size: 5
  skip_percent: 0
`
	_, err = configFile.Write([]byte(configData))
	require.NoError(t, err)
	configFile.Close()
	// Test component creation
	component, err := newSyslogComponent(configFile.Name(), "", 0)
	require.NoError(t, err)
	require.NotNil(t, component)

	// Test component name
	assert.Equal(t, consts.ComponentNameSyslog, component.Name())

	// Test timeout
	timeout := component.GetTimeout()
	fmt.Println("timeout:", timeout)
	assert.Equal(t, 60*time.Second, timeout)
}

func TestSyslogComponentWithNVLSError(t *testing.T) {
	// Create temporary log file with NVLS errors
	logFile, err := os.CreateTemp("", "fabricmanager_*.log")
	require.NoError(t, err)
	defer os.Remove(logFile.Name())

	// Write NVLS error logs to the file
	nvlsErrorLogs := `[Sep 26 2025 19:32:39] [ERROR] [tid 1569349] failed to find the member GPU handle 2977498750880931997 in partition -1 in the multicast team setup request id 1758886358995437.
[Sep 26 2025 19:32:46] [INFO] [tid 1569303] Sending inband response message: Message header details: magic Id:adbc request Id:63fb2a4a0d1ed status:57 type:3 length:24
[Sep 26 2025 19:32:46] [ERROR] [tid 1569349] failed to find the member GPU handle 2977498750880931997 in partition -1 in the multicast team setup request id 1758886366850788.
`

	_, err = logFile.Write([]byte(nvlsErrorLogs))
	require.NoError(t, err)
	logFile.Close()
	// Create temporary config file
	configFile, err := os.CreateTemp("", "syslog_cfg_*.yaml")
	require.NoError(t, err)
	defer os.Remove(configFile.Name())

	configData := `
syslog:
  query_interval: 30s
  cache_size: 5
  skip_percent: 0
`
	_, err = configFile.Write([]byte(configData))
	require.NoError(t, err)
	configFile.Close()
	// Create temporary spec file with NVLS error rule
	specFile, err := os.CreateTemp("", "syslog_spec_*.yaml")
	require.NoError(t, err)
	defer os.Remove(specFile.Name())

	specData := `
syslog:
  NVLSError:
    name: "NVLSError"
    log_file: "` + logFile.Name() + `"
    regexp: "failed to find the member GPU handle .* in the multicast team setup request id .*"
    description: "NVLS error detected"
    level: "error"
    suggestion: "Check NVLS status"
`
	_, err = specFile.Write([]byte(specData))
	require.NoError(t, err)
	specFile.Close()
	// Create component
	fmt.Printf("Test SyslogComponent with configFile: %s, specFile: %s, logFile: %s\n", configFile.Name(), specFile.Name(), logFile.Name())
	component, err := newSyslogComponent(configFile.Name(), specFile.Name(), 0)
	require.NoError(t, err)
	require.NotNil(t, component)

	// Test health check - should detect NVLS errors
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := component.HealthCheck(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// The result should indicate abnormal status due to NVLS errors
	assert.Equal(t, consts.StatusAbnormal, result.Status)

	// Check that NVLS error checker is present in the result
	var nvlsCheckerFound bool
	for _, checker := range result.Checkers {
		if checker.Name == "NVLSError" {
			nvlsCheckerFound = true
			assert.Equal(t, consts.StatusAbnormal, checker.Status)
			assert.Contains(t, checker.ErrorName, "NVLSError")
			assert.Equal(t, checker.Curr, "2")
			break
		}
	}
	assert.True(t, nvlsCheckerFound, "NVLSError checker should be found in results")

	// Test PrintInfo with NVLS errors
	passed := component.PrintInfo(nil, result, true)
	assert.False(t, passed) // Should return false due to NVLS errors

}

func TestSyslogComponentWithConntiniousNVLSError(t *testing.T) {
	// Create temporary log file with NVLS errors
	logFile, err := os.CreateTemp("", "fabricmanager_*.log")
	require.NoError(t, err)
	defer os.Remove(logFile.Name())

	// do not close logFile, continuous write
	writer, err := os.OpenFile(logFile.Name(), os.O_APPEND|os.O_WRONLY, 0666)
	require.NoError(t, err)
	defer writer.Close()

	// background goroutine to simulate continuous log writing
	stopCh := make(chan struct{})
	go func() {
		i := 0
		for {
			select {
			case <-stopCh:
				return
			default:
				i++
				t.Logf("write line %d", i)
				line := fmt.Sprintf("[Sep 26 2025 19:32:%02d] [ERROR] [tid %d] failed to find the member GPU handle %d in partition -1 in the multicast team setup request id %d.\n",
					i, 1000+i, 2000+i, 3000+i)
				_, _ = writer.WriteString(line)
				writer.Sync()
				time.Sleep(10 * time.Second) // write a line every 10s
			}
		}
	}()

	// Create component
	// Create temporary spec file with NVLS error rule
	specFile, err := os.CreateTemp("", "syslog_spec_*.yaml")
	require.NoError(t, err)
	defer os.Remove(specFile.Name())

	specData := `
syslog:
  NVLSError:
    name: "NVLSError"
    log_file: "` + logFile.Name() + `"
    regexp: "failed to find the member GPU handle .* in the multicast team setup request id .*"
    description: "NVLS error detected"
    level: "error"
    suggestion: "Check NVLS status"
`
	_, err = specFile.Write([]byte(specData))
	require.NoError(t, err)
	specFile.Close()
	component, err := newSyslogComponent("", specFile.Name(), -1)
	require.NoError(t, err)
	require.NotNil(t, component)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// multiple healthcheck, verify can continuously detect new errors
	var lastCurr string
	for i := 0; i < 2; i++ {
		result, err := component.HealthCheck(ctx)
		require.NoError(t, err)
		require.NotNil(t, result)

		for _, checker := range result.Checkers {
			if checker.Name == "NVLSError" {
				t.Logf("iteration=%d, Curr=%s", i, checker.Curr)
				// verify Curr is increasing
				if lastCurr != "" {
					require.Greater(t, checker.Curr, lastCurr, "Curr should keep increasing")
				}
				lastCurr = checker.Curr
			}
		}
		time.Sleep(consts.DefaultFileLoaderInterval + 5*time.Second)
	}

	// stop log writing goroutine
	close(stopCh)
}
