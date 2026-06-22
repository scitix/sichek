package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockInfo struct {
	Value string `json:"value"`
}

func (m *MockInfo) JSON() (string, error) {
	data, err := json.Marshal(m)
	return string(data), err
}

func TestSnapshotManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	snapshotPath := filepath.Join(tmpDir, "snapshot.json")

	// Create a dummy config file
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgFile, []byte("snapshot:\n  enable: true\n  path: "+snapshotPath), 0644)

	mgr, err := NewSnapshotManager(cfgFile)
	assert.NoError(t, err)
	assert.Equal(t, snapshotPath, mgr.path)
	assert.True(t, mgr.enabled)

	// Update component 1
	info1 := &MockInfo{Value: "cpu-data"}
	mgr.Update("cpu", info1)

	// Verify file exists and content is correct
	data, err := os.ReadFile(snapshotPath)
	assert.NoError(t, err)

	var snapshot Snapshot
	err = json.Unmarshal(data, &snapshot)
	assert.NoError(t, err)
	assert.Contains(t, snapshot.Components, "cpu")

	// Complex check: the component data in snapshot.Components[cpu] will be a map[string]interface{} after unmarshaling
	cpuMap := snapshot.Components["cpu"].(map[string]interface{})
	assert.Equal(t, "cpu-data", cpuMap["value"])

	// Update component 2
	info2 := &MockInfo{Value: "nvidia-data"}
	mgr.Update("nvidia", info2)

	data, err = os.ReadFile(snapshotPath)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &snapshot)
	assert.NoError(t, err)
	assert.Contains(t, snapshot.Components, "cpu")
	assert.Contains(t, snapshot.Components, "nvidia")
}

func TestSnapshotManager_SetIssues(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.json")
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("snapshot:\n  enable: true\n  path: "+snapshotPath), 0644))

	mgr, err := NewSnapshotManager(cfgFile)
	require.NoError(t, err)

	// Accumulate an issue via the store, then mirror it into the snapshot.
	store := NewAnnotationStore()
	anno, err := store.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "CPUCheck", "ErrA", "cpu0"))
	require.NoError(t, err)
	mgr.SetIssues(anno)

	data, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)

	var snapshot Snapshot
	require.NoError(t, json.Unmarshal(data, &snapshot))

	// The top-level issues field is present and carries the CPU issue.
	require.NotNil(t, snapshot.Issues)
	assert.ElementsMatch(t, []string{"ErrA"}, errorNames(snapshot.Issues.CPU, consts.LevelCritical))

	// The serialized snapshot exposes the top-level "issues" key.
	assert.Contains(t, string(data), `"issues"`)
}
