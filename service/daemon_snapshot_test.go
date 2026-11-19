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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lastInfoStub implements only LastInfo; updateSnapshotInfo needs nothing else.
type lastInfoStub struct {
	info common.Info
	err  error
}

func (s lastInfoStub) LastInfo() (common.Info, error) { return s.info, s.err }

func newTestSnapshotMgr(t *testing.T) *SnapshotManager {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte("snapshot:\n  enable: true\n  path: "+filepath.Join(dir, "snapshot.json")), 0644))
	mgr, err := NewSnapshotManager(cfg)
	require.NoError(t, err)
	return mgr
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldOut, oldLevel := logrus.StandardLogger().Out, logrus.GetLevel()
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		logrus.SetOutput(oldOut)
		logrus.SetLevel(oldLevel)
	})
	return &buf
}

func TestUpdateSnapshotInfo(t *testing.T) {
	tests := []struct {
		name        string
		stub        lastInfoStub
		wantStored  bool
		wantLevel   string // substring expected in the log, "" means no warn/error output
		wantNoLevel []string
	}{
		{
			name:        "component provides no info (sentinel) is silent and skipped",
			stub:        lastInfoStub{err: common.ErrNoInfo},
			wantStored:  false,
			wantNoLevel: []string{"level=error", "level=warning"},
		},
		{
			name:        "wrapped sentinel is also silent",
			stub:        lastInfoStub{err: fmt.Errorf("syslog: %w", common.ErrNoInfo)},
			wantStored:  false,
			wantNoLevel: []string{"level=error", "level=warning"},
		},
		{
			name:        "nil info without error is silent and skipped",
			stub:        lastInfoStub{},
			wantStored:  false,
			wantNoLevel: []string{"level=error", "level=warning"},
		},
		{
			name:       "real error is still logged at error level",
			stub:       lastInfoStub{err: errors.New("collector exploded")},
			wantStored: false,
			wantLevel:  "level=error",
		},
		{
			name:        "real info is stored",
			stub:        lastInfoStub{info: &MockInfo{Value: "cpu-data"}},
			wantStored:  true,
			wantNoLevel: []string{"level=error", "level=warning"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestSnapshotMgr(t)
			logs := captureLogs(t)

			updateSnapshotInfo(mgr, "comp", tt.stub)

			_, stored := mgr.data.Components["comp"]
			assert.Equal(t, tt.wantStored, stored)
			if tt.wantLevel != "" {
				assert.Contains(t, logs.String(), tt.wantLevel)
				assert.Contains(t, logs.String(), "collector exploded")
			}
			for _, lvl := range tt.wantNoLevel {
				assert.NotContains(t, logs.String(), lvl)
			}
		})
	}
}
