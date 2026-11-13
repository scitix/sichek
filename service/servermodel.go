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
	"syscall"
	"time"
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
