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
package collector

import (
	"context"
	"encoding/json"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/sirupsen/logrus"
)

type Output struct {
	Info *MemoryInfo `json:"info"`
	Time time.Time
}

func (o *Output) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}

type MemoryCollector struct {
	name string
}

func NewCollector() (*MemoryCollector, error) {
	return &MemoryCollector{
		name: "MemoryCollector",
	}, nil
}

func (c *MemoryCollector) Name() string {
	return c.name
}

func (c *MemoryCollector) Collect(ctx context.Context) (common.Info, error) {
	info := &MemoryInfo{}
	err := info.Get()
	if err != nil {
		logrus.WithField("collector", "memory").Errorf("get memory info failed: %v", err)
		return nil, err
	}

	output := &Output{
		Info: info,
		Time: time.Now(),
	}

	return output, nil
}
