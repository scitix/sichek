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
	"path"
	"runtime"
	"testing"
	"time"
)

func TestGpfs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Log("get curr file path failed")
		return
	}
	testCfgFile := path.Dir(curFile) + "/test/gpfsCfg.yaml"

	component, err := NewGpfsComponent(testCfgFile)
	if err != nil {
		t.Log(err)
		return
	}

	result, err := component.HealthCheck(ctx)
	if err != nil {
		t.Log(err)
	}
	result_str, err := result.JSON()
	if err != nil {
		t.Log(err)
	}
	t.Logf("test gpfs analysis result: %s", result_str)
	t.Logf("Running time: %ds", time.Since(start))
}
