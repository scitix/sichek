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
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const catcherFull = `{"model_id":"I57G90N13M63S10","components":{
 "cpu":{"code":"I57","detected":"INTEL(R) XEON(R) GOLD 6530 (2 socket)","key":"Intel 6530|2"},
 "gpu":{"code":"G90","detected":"NVIDIA GeForce RTX 5090 *8","key":"RTX5090|8"},
 "nic":{"code":"N13","detected":"100Gbps*2","key":"100Gx2*1"},
 "mem":{"code":"M63","detected":"DDR5 64G *16","key":"64|16"},
 "disk":{"code":"S10","detected":"SSD 960G*2 + SSD 3.84T*2","key":"960G*2+3.84T*2"}},
 "config":"CPU: Intel 6530 *2\nGPU: 5090 *8"}`

const catcherMissingDisk = `{"model_id":"I57G90N13M63","components":{
 "cpu":{"code":"I57","detected":"x","key":"Intel 6530|2"},
 "gpu":{"code":"G90","detected":"x","key":"RTX5090|8"},
 "nic":{"code":"N13","detected":"x","key":"100Gx2*1"},
 "mem":{"code":"M63","detected":"x","key":"64|16"},
 "disk":{"code":"","detected":"SSD 894G*1","key":"894G*1"}},"config":""}`

const catcherNoGPU = `{"model_id":"I57N13M63S10","components":{
 "cpu":{"code":"I57","detected":"x","key":"k"},
 "gpu":{"code":"","detected":"","key":""},
 "nic":{"code":"N13","detected":"x","key":"k"},
 "mem":{"code":"M63","detected":"x","key":"k"},
 "disk":{"code":"S10","detected":"x","key":"k"}},"config":""}`

const catcherUnmatchedGPU = `{"model_id":"I57N13M63S10","components":{
 "cpu":{"code":"I57","detected":"x","key":"k"},
 "gpu":{"code":"","detected":"NVIDIA B300 *8","key":"RAW:NVIDIA B300|8"},
 "nic":{"code":"N13","detected":"x","key":"k"},
 "mem":{"code":"M63","detected":"x","key":"k"},
 "disk":{"code":"S10","detected":"x","key":"k"}},"config":""}`

const catcherEmptyID = `{"model_id":"","components":{
 "cpu":{"code":"","detected":"","key":""},
 "gpu":{"code":"","detected":"","key":""},
 "nic":{"code":"","detected":"","key":""},
 "mem":{"code":"","detected":"","key":""},
 "disk":{"code":"","detected":"","key":""}},"config":""}`

func TestParseCatcherResult(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantID       string
		wantComplete bool
		wantMissing  []string
		wantErr      bool
	}{
		{name: "complete", input: catcherFull, wantID: "I57G90N13M63S10", wantComplete: true, wantMissing: []string{}},
		{name: "disk unmatched", input: catcherMissingDisk, wantID: "I57G90N13M63", wantComplete: false, wantMissing: []string{"disk"}},
		{name: "no gpu present", input: catcherNoGPU, wantID: "I57N13M63S10", wantComplete: true, wantMissing: []string{}},
		{name: "gpu present but unmatched", input: catcherUnmatchedGPU, wantID: "I57N13M63S10", wantComplete: false, wantMissing: []string{"gpu"}},
		{name: "empty model id", input: catcherEmptyID, wantID: "", wantComplete: false, wantMissing: []string{"cpu", "nic", "mem", "disk"}},
		{name: "corrupt json", input: `{"model_id":`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, sm, err := parseCatcherResult([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantComplete, sm.Complete)
			assert.Equal(t, tt.wantMissing, sm.Missing)
		})
	}
}

func TestParseCatcherResult_KeepsComponentsAndConfig(t *testing.T) {
	_, sm, err := parseCatcherResult([]byte(catcherFull))
	require.NoError(t, err)
	assert.Equal(t, ServerModelPart{Code: "I57", Detected: "INTEL(R) XEON(R) GOLD 6530 (2 socket)", Key: "Intel 6530|2"}, sm.Components["cpu"])
	assert.Len(t, sm.Components, 5)
	assert.Equal(t, "CPU: Intel 6530 *2\nGPU: 5090 *8", sm.Config)
}
