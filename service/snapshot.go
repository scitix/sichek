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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

// SnapshotConfig represents the configuration for snapshotting component information.
type SnapshotConfig struct {
	Snapshot struct {
		Enable bool   `json:"enable" yaml:"enable"`
		Path   string `json:"path" yaml:"path"`
	} `json:"snapshot" yaml:"snapshot"`
}

// Snapshot represents the aggregated data from all components.
type Snapshot struct {
	Node   string `json:"node"`
	MgmtIP string `json:"mgmt_ip,omitempty"`
	// BootTime is the node's boot instant. It is stable for the life of the
	// process, so it is computed once at startup.
	BootTime time.Time `json:"boot_time,omitempty"`
	// UptimeSeconds is the node's uptime in seconds, refreshed to the current
	// value on every persist.
	UptimeSeconds float64                `json:"uptime_seconds,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	Components    map[string]interface{} `json:"components"`
	// Issues mirrors the K8s node annotation: detected problems grouped by
	// component then level. It lets the collector consume issues without reading
	// the K8s annotation, and is populated even on non-K8s nodes.
	Issues *nodeAnnotation `json:"issues"`
}

// SnapshotManager manages the aggregation and persistence of component information.
type SnapshotManager struct {
	mu       sync.RWMutex
	data     *Snapshot
	path     string
	enabled  bool
	nodeName string
}

// NewSnapshotManager creates a new SnapshotManager.
func NewSnapshotManager(cfgFile string) (*SnapshotManager, error) {
	config := &SnapshotConfig{}
	// Set defaults
	config.Snapshot.Enable = true
	config.Snapshot.Path = consts.DefaultSnapshotPath

	if cfgFile != "" {
		err := utils.LoadFromYaml(cfgFile, config)
		if err != nil {
			logrus.WithField("service", "snapshot").Warnf("Failed to load snapshot config from %s, using defaults: %v", cfgFile, err)
		}
	}

	hostname, _ := os.Hostname()
	bootTime, err := utils.GetBootTime()
	if err != nil {
		logrus.WithField("service", "snapshot").Warnf("Failed to read boot time: %v", err)
	}
	mgr := &SnapshotManager{
		path:     config.Snapshot.Path,
		enabled:  config.Snapshot.Enable,
		nodeName: hostname,
		data: &Snapshot{
			Node:       hostname,
			MgmtIP:     utils.GetMgmtIP(),
			BootTime:   bootTime,
			Components: make(map[string]interface{}),
		},
	}

	if mgr.enabled {
		logrus.WithField("service", "snapshot").Infof("Snapshot manager enabled, path: %s", mgr.path)
	}

	return mgr, nil
}

// Update updates the snapshot with information from a component.
func (s *SnapshotManager) Update(componentName string, info common.Info) {
	if !s.enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Timestamp = time.Now()
	// common.Info has JSON() method, we can use it or just store the object directly
	// Marshaling/unmarshaling is a safe way to ensure we have a clean JSON-serializable map
	s.data.Components[componentName] = info

	if err := s.persist(); err != nil {
		logrus.WithField("service", "snapshot").Errorf("Failed to persist snapshot: %v", err)
	}
}

// SetIssues updates the snapshot's issue list (the mirror of the node annotation)
// and persists. The issues object is node-global, so it is replaced wholesale on
// each call rather than merged per component.
func (s *SnapshotManager) SetIssues(issues *nodeAnnotation) {
	if !s.enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Timestamp = time.Now()
	s.data.Issues = issues

	if err := s.persist(); err != nil {
		logrus.WithField("service", "snapshot").Errorf("Failed to persist snapshot: %v", err)
	}
}

// persist writes the current snapshot to the local JSON file atomically.
// It refreshes UptimeSeconds to the current value before marshaling; a read
// failure leaves the previous value untouched rather than blocking the write.
func (s *SnapshotManager) persist() error {
	if uptime, err := utils.GetUptime(); err != nil {
		logrus.WithField("service", "snapshot").Warnf("Failed to read uptime: %v", err)
	} else {
		s.data.UptimeSeconds = uptime
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		// A component's Info may contain a non-finite float (Inf/NaN) — e.g. a
		// transceiver lane reporting "-inf dBm" — which encoding/json rejects
		// atomically, failing the marshal of the whole snapshot. Without a
		// fallback this freezes snapshot.json indefinitely: every subsequent
		// persist hits the same value and the file keeps its last-good content.
		// Re-encode with each component marshaled independently so the offending
		// one is replaced by a placeholder while node identity, uptime, issues,
		// and every healthy component still persist.
		logrus.WithField("service", "snapshot").Warnf("marshal snapshot failed (%v); retrying with per-component isolation", err)
		data, err = s.marshalIsolatingBadComponents()
		if err != nil {
			return fmt.Errorf("marshal snapshot failed: %w", err)
		}
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s failed: %w", dir, err)
	}

	tmpFile := s.path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write tmp file failed: %w", err)
	}

	if err := os.Rename(tmpFile, s.path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename %s to %s failed: %w", tmpFile, s.path, err)
	}

	return nil
}

// marshalIsolatingBadComponents re-encodes the snapshot when a whole-snapshot
// marshal failed. Each component is marshaled on its own; any that fails (e.g.
// because it holds a non-finite float) is swapped for an error placeholder so
// that a single bad component can no longer block the entire snapshot. All
// other top-level fields (node, mgmt_ip, boot_time, uptime_seconds, timestamp,
// issues) are preserved by embedding the original snapshot and shadowing only
// its Components field.
func (s *SnapshotManager) marshalIsolatingBadComponents() ([]byte, error) {
	safeComponents := make(map[string]json.RawMessage, len(s.data.Components))
	for name, info := range s.data.Components {
		raw, err := json.Marshal(info)
		if err != nil {
			logrus.WithField("service", "snapshot").Warnf("component %q not JSON-serializable (%v); storing placeholder", name, err)
			raw, _ = json.Marshal(map[string]string{"_snapshot_error": err.Error()})
		}
		safeComponents[name] = raw
	}

	// alias drops Snapshot's methods so the embedded value is marshaled as plain
	// data; the outer Components field (depth 0) shadows the embedded one.
	type alias Snapshot
	envelope := struct {
		*alias
		Components map[string]json.RawMessage `json:"components"`
	}{
		alias:      (*alias)(s.data),
		Components: safeComponents,
	}
	return json.MarshalIndent(envelope, "", "  ")
}
