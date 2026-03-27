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
	Node       string                 `json:"node"`
	Timestamp  time.Time              `json:"timestamp"`
	Components map[string]interface{} `json:"components"`
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
	mgr := &SnapshotManager{
		path:     config.Snapshot.Path,
		enabled:  config.Snapshot.Enable,
		nodeName: hostname,
		data: &Snapshot{
			Node:       hostname,
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

// persist writes the current snapshot to the local JSON file atomically.
func (s *SnapshotManager) persist() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot failed: %w", err)
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
