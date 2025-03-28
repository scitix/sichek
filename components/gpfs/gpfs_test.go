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
package gpfs

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/config"
)

func TestGpfs_HealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a temporary log file for testing in a goroutine
	var logFile *os.File
	var err error
	testLogFile := "/tmp/test_mmfs_log"
	logFile, err = os.Create(testLogFile)
	if err != nil {
		t.Fatalf("Failed to create temp test_mmfs_log file: %v", err)
	}
	t.Logf("Log file: %s", logFile.Name())
	// Write some test data to the log file
	_, err = logFile.WriteString("test log data\n")
	if err != nil {
		t.Fatalf("Failed to write to temp log file: %+v", err)
	}
	logFile.Close()

	// NewGpfsComponent
	start := time.Now()

	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Log("get curr file path failed")
		return
	}
	testCfgFile := path.Dir(curFile) + "/test/gpfsCfg.yaml"
	componentConfig, err := config.LoadComponentConfig(testCfgFile, "")
	if err != nil {
		t.Log(err)
		return
	}
	component, err := NewGpfsComponent(componentConfig)
	if err != nil {
		t.Log(err)
		return
	}

	// Start a goroutine to write the to the context of `test/gpfsCfg.yaml` to logFile continuously
	// Read the current content of the log file
	testLog := path.Dir(curFile) + "/test/test_mmfs_log"
	content, err := os.ReadFile(testLog)
	if err != nil {
		t.Fatalf("Failed to read log file0: %v", err)
	}
	// t.Logf("Current log file content: %s", string(content))
	go func() {
		for {
			select {
			case <-ctx.Done():
				os.Remove(logFile.Name())
				return
			default:

				file, err := os.OpenFile(logFile.Name(), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Errorf("Failed to open log file: %v", err)
					return
				}
				_, err = file.WriteString(fmt.Sprintf("%s\n", string(content)))
				if err != nil {
					t.Errorf("Failed to write to log file: %v", err)
					file.Close()
					return
				}
				file.Close()
				time.Sleep(1 * time.Second)
			}
		}
	}()
	time.Sleep(1 * time.Second)
	content, err = os.ReadFile(testLogFile)
	if err != nil {
		t.Fatalf("Failed to read testLogFile: %v", err)
	}
	t.Logf("Current log file content: %s", string(content))

	result, err := component.HealthCheck(ctx)
	if err != nil {
		t.Log(err)
	}
	t.Logf("test gpfs analysis result: %s", common.ToString(result))
	t.Logf("Running time: %ds", time.Since(start))
}
