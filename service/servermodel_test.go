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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func setGPUPresent(t *testing.T, v bool) {
	t.Helper()
	old := nvidiaGPUPresent
	nvidiaGPUPresent = func() bool { return v }
	t.Cleanup(func() { nvidiaGPUPresent = old })
}

const catcherNoJQConfig = `{"model_id":"I57G90N13M63S10","components":{
 "cpu":{"code":"I57","detected":"x","key":"k"},
 "gpu":{"code":"G90","detected":"x","key":"k"},
 "nic":{"code":"N13","detected":"x","key":"k"},
 "mem":{"code":"M63","detected":"x","key":"k"},
 "disk":{"code":"S10","detected":"x","key":"k"}},"config":"CPU: Intel 6530 *2#GPU: 5090 *8#"}`

func TestHasNvidiaGPU(t *testing.T) {
	tests := []struct {
		name          string
		vendor, class string
		noDevices     bool
		want          bool
	}{
		{name: "nvidia 3D controller", vendor: "0x10de", class: "0x030200", want: true},
		{name: "nvidia VGA", vendor: "0x10de", class: "0x030000", want: true},
		{name: "nvidia audio", vendor: "0x10de", class: "0x040300", want: false},
		{name: "mellanox", vendor: "0x15b3", class: "0x020000", want: false},
		{name: "empty dir", noDevices: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			devs := filepath.Join(root, "bus", "pci", "devices")
			require.NoError(t, os.MkdirAll(devs, 0755))
			if !tt.noDevices {
				d := filepath.Join(devs, "0000:17:00.0")
				require.NoError(t, os.MkdirAll(d, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(d, "vendor"), []byte(tt.vendor+"\n"), 0644))
				require.NoError(t, os.WriteFile(filepath.Join(d, "class"), []byte(tt.class+"\n"), 0644))
			}
			assert.Equal(t, tt.want, hasNvidiaGPU(root))
		})
	}
	assert.False(t, hasNvidiaGPU(filepath.Join(t.TempDir(), "missing")))
}

func TestParseCatcherResult_GPUQueryFailure(t *testing.T) {
	setGPUPresent(t, true)
	_, sm, err := parseCatcherResult([]byte(catcherNoGPU))
	require.NoError(t, err)
	assert.False(t, sm.Complete)
	assert.Equal(t, []string{"gpu"}, sm.Missing)

	setGPUPresent(t, false)
	_, sm, err = parseCatcherResult([]byte(catcherNoGPU))
	require.NoError(t, err)
	assert.True(t, sm.Complete)
	assert.Equal(t, []string{}, sm.Missing)
}

func TestParseCatcherResult_NoJQConfigNormalized(t *testing.T) {
	_, sm, err := parseCatcherResult([]byte(catcherNoJQConfig))
	require.NoError(t, err)
	assert.Equal(t, "CPU: Intel 6530 *2\nGPU: 5090 *8\n", sm.Config)
}

func TestParseCatcherResult(t *testing.T) {
	setGPUPresent(t, false)
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

// writeFakeCatcher creates a fake catcher dir with the given script body and
// returns the path to catcher.bash. A lib/ file is included to verify the whole
// directory (not just the entry script) is copied.
func writeFakeCatcher(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "data.json"), []byte(catcherFull), 0644))
	script := filepath.Join(dir, "catcher.bash")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/bash\n"+body), 0755))
	return script
}

func TestRunCatcher_Success(t *testing.T) {
	// Mirrors real catcher: writes into its own dir and only prints a path.
	script := writeFakeCatcher(t, `D="$(cd "$(dirname "$0")" && pwd)"
[ "$1" = json ] || exit 9
mkdir -p "$D/result" && cp "$D/lib/data.json" "$D/result/catcher_result.json"
echo "JSON written: $D/result/catcher_result.json"`)
	srcDir := filepath.Dir(script)

	data, err := runCatcher(context.Background(), script, 10*time.Second)
	require.NoError(t, err)
	id, _, err := parseCatcherResult(data)
	require.NoError(t, err)
	assert.Equal(t, "I57G90N13M63S10", id)

	// The install dir must stay untouched (it may be read-only in production).
	_, statErr := os.Stat(filepath.Join(srcDir, "result"))
	assert.True(t, os.IsNotExist(statErr), "catcher must run in a temp copy, not the install dir")
}

func TestRunCatcher_NoResultFile(t *testing.T) {
	script := writeFakeCatcher(t, `exit 0`)
	_, err := runCatcher(context.Background(), script, 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catcher_result.json")
}

func TestRunCatcher_NonZeroExit(t *testing.T) {
	script := writeFakeCatcher(t, `echo boom >&2; exit 3`)
	_, err := runCatcher(context.Background(), script, 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRunCatcher_MissingScript(t *testing.T) {
	_, err := runCatcher(context.Background(), filepath.Join(t.TempDir(), "nope", "catcher.bash"), 10*time.Second)
	require.Error(t, err)
}

func TestRunCatcher_TimeoutKillsProcessGroup(t *testing.T) {
	// A backgrounded grandchild keeps stdout open; without a process-group
	// kill, waiting for output would block until it exits (60s).
	script := writeFakeCatcher(t, `sleep 60 &
sleep 60`)
	start := time.Now()
	_, err := runCatcher(context.Background(), script, 500*time.Millisecond)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 3*time.Second)
}

func TestReadCatcherVersion(t *testing.T) {
	script := writeFakeCatcher(t, "exit 0")
	assert.Equal(t, "", readCatcherVersion(script))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(script), "VERSION"), []byte("sha256:abc 2026-11-13\n"), 0644))
	assert.Equal(t, "sha256:abc 2026-11-13", readCatcherVersion(script))
}

func TestLoadServerModelConfig(t *testing.T) {
	def := defaultServerModelConfig()
	tests := []struct {
		name string
		yaml string
		want ServerModelConfig
	}{
		{name: "no block", yaml: "snapshot:\n  enable: true\n", want: def},
		{name: "partial block", yaml: "server_model:\n  interval: 30m\n", want: func() ServerModelConfig {
			c := def
			c.Interval = 30 * time.Minute
			return c
		}()},
		{name: "zero durations fall back", yaml: "server_model:\n  enable: true\n  interval: 0s\n  timeout: 0s\n  initial_delay: 0s\n", want: def},
		{name: "disabled", yaml: "server_model:\n  enable: false\n", want: func() ServerModelConfig {
			c := def
			c.Enable = false
			return c
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "cfg.yaml")
			require.NoError(t, os.WriteFile(f, []byte(tt.yaml), 0644))
			got, err := LoadServerModelConfig(f)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultServerModelConfig(t *testing.T) {
	c := defaultServerModelConfig()
	assert.True(t, c.Enable)
	assert.Equal(t, "/var/sichek/scripts/catcher/catcher.bash", c.ScriptPath)
	assert.Equal(t, 5*time.Minute, c.InitialDelay)
	assert.Equal(t, time.Hour, c.Interval)
	assert.Equal(t, 120*time.Second, c.Timeout)
}

// newTestSnapshot returns an enabled SnapshotManager writing to a temp file.
func newTestSnapshot(t *testing.T) (*SnapshotManager, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	cfg := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte("snapshot:\n  enable: true\n  path: "+path), 0644))
	mgr, err := NewSnapshotManager(cfg)
	require.NoError(t, err)
	return mgr, path
}

func readSnapshot(t *testing.T, path string) Snapshot {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var s Snapshot
	require.NoError(t, json.Unmarshal(data, &s))
	return s
}

const fakeCatcherOK = `D="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$D/result" && cp "$D/lib/data.json" "$D/result/catcher_result.json"`

func TestServerModelRunner_SuccessThenFailureKeepsLastGood(t *testing.T) {
	script := writeFakeCatcher(t, fakeCatcherOK)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(script), "VERSION"), []byte("sha256:abc 2026-11-13\n"), 0644))
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = script
	cfg.Timeout = 10 * time.Second

	var gotID string
	var gotComplete bool
	calls := 0
	r := NewServerModelRunner(cfg, snap, func(id string, complete bool) { gotID, gotComplete, calls = id, complete, calls+1 })

	r.RunOnce(context.Background())
	s := readSnapshot(t, path)
	assert.Equal(t, "I57G90N13M63S10", s.ServerModelID)
	require.NotNil(t, s.ServerModel)
	assert.True(t, s.ServerModel.Complete)
	assert.Equal(t, "sha256:abc 2026-11-13", s.ServerModel.CatcherVersion)
	assert.Empty(t, s.ServerModel.Error)
	assert.False(t, s.ServerModel.CollectedAt.IsZero())
	assert.Equal(t, "I57G90N13M63S10", gotID)
	assert.True(t, gotComplete)
	firstAt := s.ServerModel.CollectedAt

	// Break the script: next run must keep the last good ID and only set error.
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/bash\nexit 1\n"), 0755))
	time.Sleep(10 * time.Millisecond)
	r.RunOnce(context.Background())
	s = readSnapshot(t, path)
	assert.Equal(t, "I57G90N13M63S10", s.ServerModelID)
	require.NotNil(t, s.ServerModel)
	assert.True(t, s.ServerModel.Complete)
	assert.NotEmpty(t, s.ServerModel.Error)
	assert.True(t, s.ServerModel.CollectedAt.After(firstAt))
	assert.Equal(t, 1, calls, "metrics callback must not fire on failure")
}

func TestServerModelRunner_ShutdownMidRunPublishesNothing(t *testing.T) {
	script := writeFakeCatcher(t, "sleep 30")
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = script
	cfg.Timeout = 30 * time.Second
	calls := 0
	r := NewServerModelRunner(cfg, snap, func(string, bool) { calls++ })

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	r.RunOnce(ctx)

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "shutdown must not publish a spurious error")
	assert.Equal(t, 0, calls)
}

func TestServerModelRunner_FirstRunFailure(t *testing.T) {
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = filepath.Join(t.TempDir(), "missing", "catcher.bash")
	calls := 0
	r := NewServerModelRunner(cfg, snap, func(string, bool) { calls++ })

	r.RunOnce(context.Background())
	s := readSnapshot(t, path)
	assert.Empty(t, s.ServerModelID)
	require.NotNil(t, s.ServerModel)
	assert.NotEmpty(t, s.ServerModel.Error)
	assert.False(t, s.ServerModel.Complete)
	assert.Equal(t, 0, calls)
}

func TestServerModelRunner_RunHonorsInitialDelayAndStops(t *testing.T) {
	script := writeFakeCatcher(t, fakeCatcherOK)
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = script
	cfg.InitialDelay = 200 * time.Millisecond
	cfg.Interval = time.Hour
	r := NewServerModelRunner(cfg, snap, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "must not run before initial delay")
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)
	assert.Equal(t, "I57G90N13M63S10", readSnapshot(t, path).ServerModelID)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}
