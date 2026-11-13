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
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ServerModelPart is one hardware part of the catcher result.
type ServerModelPart struct {
	Code     string `json:"code"`
	Detected string `json:"detected"`
	Key      string `json:"key"`
}

// ServerModel is the detail behind server_model_id, produced by the hardware
// team's catcher script. It is node metadata, not a health check.
type ServerModel struct {
	Complete       bool                       `json:"complete"`
	Missing        []string                   `json:"missing"`
	Components     map[string]ServerModelPart `json:"components,omitempty"`
	Config         string                     `json:"config,omitempty"`
	CatcherVersion string                     `json:"catcher_version,omitempty"`
	CollectedAt    time.Time                  `json:"collected_at"`
	Error          string                     `json:"error"`
}

// catcherResult mirrors result/catcher_result.json written by `catcher.bash json`.
type catcherResult struct {
	ModelID    string                     `json:"model_id"`
	Components map[string]ServerModelPart `json:"components"`
	Config     string                     `json:"config"`
}

// serverModelParts is the catcher's fixed ID order (CPU → GPU → NIC → MEM → DISK).
var serverModelParts = []string{"cpu", "gpu", "nic", "mem", "disk"}

// parseCatcherResult decodes a catcher result and derives Complete/Missing.
// A part is missing when it has no code; GPU is the exception — an empty GPU
// part with nothing detected is a GPU-less node, which catcher itself does not
// count as incomplete.
func parseCatcherResult(data []byte) (string, *ServerModel, error) {
	var r catcherResult
	if err := json.Unmarshal(data, &r); err != nil {
		return "", nil, fmt.Errorf("parse catcher result: %w", err)
	}
	missing := []string{}
	for _, name := range serverModelParts {
		p := r.Components[name]
		if p.Code != "" {
			continue
		}
		if name == "gpu" && p.Detected == "" && p.Key == "" {
			continue
		}
		missing = append(missing, name)
	}
	return r.ModelID, &ServerModel{
		Complete:   r.ModelID != "" && len(missing) == 0,
		Missing:    missing,
		Components: r.Components,
		Config:     r.Config,
	}, nil
}

// runCatcher copies the catcher directory to a temp dir (catcher writes
// result/ next to itself and the install dir may be read-only), runs
// `catcher.bash json` in its own process group, and returns the result file.
// On timeout the whole group is killed: catcher spawns nvidia-smi/dmidecode,
// and a plain context kill only reaches bash, leaving grandchildren holding
// the output pipe open.
func runCatcher(ctx context.Context, scriptPath string, timeout time.Duration) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "sichek-catcher-")
	if err != nil {
		return nil, fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := copyDir(filepath.Dir(scriptPath), tmp); err != nil {
		return nil, fmt.Errorf("copy catcher: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(tmp, filepath.Base(scriptPath)), "json")
	cmd.Dir = tmp
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("catcher timed out after %s", timeout)
		}
		return nil, fmt.Errorf("catcher failed: %w: %s", err, strings.TrimSpace(tail(out.String(), 512)))
	}
	data, err := os.ReadFile(filepath.Join(tmp, "result", "catcher_result.json"))
	if err != nil {
		return nil, fmt.Errorf("read catcher_result.json: %w", err)
	}
	return data, nil
}

// readCatcherVersion returns the VERSION file next to the script, or "".
func readCatcherVersion(scriptPath string) string {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(scriptPath), "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// copyDir copies regular files and directories from src into dst, skipping
// any stale result/ directory.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() && rel == "result" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0755)
	})
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ServerModelConfig controls the periodic catcher run.
type ServerModelConfig struct {
	Enable       bool          `json:"enable"        yaml:"enable"`
	ScriptPath   string        `json:"script_path"   yaml:"script_path"`
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`
	Interval     time.Duration `json:"interval"      yaml:"interval"`
	Timeout      time.Duration `json:"timeout"       yaml:"timeout"`
}

type serverModelFile struct {
	ServerModel ServerModelConfig `json:"server_model" yaml:"server_model"`
}

// defaultServerModelConfig: first run 5m after start (clear of the startup
// NVML burst), then hourly.
func defaultServerModelConfig() ServerModelConfig {
	return ServerModelConfig{
		Enable:       true,
		ScriptPath:   "/var/sichek/scripts/catcher/catcher.bash",
		InitialDelay: 5 * time.Minute,
		Interval:     time.Hour,
		Timeout:      120 * time.Second,
	}
}

// LoadServerModelConfig parses the server_model block from cfgFile, filling
// zero-valued fields with defaults. Missing file or block yields defaults.
func LoadServerModelConfig(cfgFile string) (ServerModelConfig, error) {
	def := defaultServerModelConfig()
	if cfgFile == "" {
		return def, nil
	}
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			return def, nil
		}
		return def, fmt.Errorf("load server_model config: %w", err)
	}
	f := serverModelFile{ServerModel: def}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return def, fmt.Errorf("load server_model config: %w", err)
	}
	out := f.ServerModel
	if out.ScriptPath == "" {
		out.ScriptPath = def.ScriptPath
	}
	if out.InitialDelay <= 0 {
		out.InitialDelay = def.InitialDelay
	}
	if out.Interval <= 0 {
		out.Interval = def.Interval
	}
	if out.Timeout <= 0 {
		out.Timeout = def.Timeout
	}
	return out, nil
}

// ServerModelRunner periodically runs catcher and publishes the result to the
// snapshot. On failure it keeps the last good ID/detail and only updates Error.
type ServerModelRunner struct {
	cfg      ServerModelConfig
	snap     *SnapshotManager
	onUpdate func(id string, complete bool)

	mu     sync.Mutex
	lastID string
	last   *ServerModel
}

// NewServerModelRunner builds a runner. onUpdate (may be nil) fires after each
// successful run, e.g. to refresh the Prometheus info metric.
func NewServerModelRunner(cfg ServerModelConfig, snap *SnapshotManager, onUpdate func(id string, complete bool)) *ServerModelRunner {
	return &ServerModelRunner{cfg: cfg, snap: snap, onUpdate: onUpdate}
}

// Run waits InitialDelay, then runs catcher every Interval until ctx is done.
func (r *ServerModelRunner) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(r.cfg.InitialDelay):
	}
	r.RunOnce(ctx)
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.RunOnce(ctx)
		}
	}
}

// RunOnce executes catcher a single time and publishes the outcome.
func (r *ServerModelRunner) RunOnce(ctx context.Context) {
	log := logrus.WithField("service", "server_model")
	now := time.Now()
	id, sm, err := r.collect(ctx)

	r.mu.Lock()
	if err != nil {
		log.Warnf("catcher run failed: %v", err)
		sm = &ServerModel{Missing: []string{}}
		if r.last != nil {
			cp := *r.last
			sm = &cp
		}
		id = r.lastID
		sm.Error = err.Error()
	} else {
		sm.CatcherVersion = readCatcherVersion(r.cfg.ScriptPath)
		r.lastID, r.last = id, sm
		log.Infof("server_model_id=%q complete=%v missing=%v", id, sm.Complete, sm.Missing)
	}
	sm.CollectedAt = now
	r.mu.Unlock()

	if r.snap != nil {
		r.snap.SetServerModel(id, sm)
	}
	if err == nil && r.onUpdate != nil {
		r.onUpdate(id, sm.Complete)
	}
}

func (r *ServerModelRunner) collect(ctx context.Context) (string, *ServerModel, error) {
	data, err := runCatcher(ctx, r.cfg.ScriptPath, r.cfg.Timeout)
	if err != nil {
		return "", nil, err
	}
	return parseCatcherResult(data)
}
