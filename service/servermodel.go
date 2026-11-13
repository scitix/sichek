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
