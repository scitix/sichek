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
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nccl/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type NCCLInfo struct {
	Time     time.Time
	Name     []string `json:"regex_name"`
	Regexp   []string `json:"regexpression"`
	FileName []string `json:"file_name"`
	Raw      []string `json:"raw_log"`
}

func (d *NCCLInfo) JSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type NCCLChecker struct {
	id   string
	name string
	cfg  *config.NCCLUserConfig
}

func NewNCCLChecker(cfg *config.NCCLUserConfig) common.Checker {
	return &NCCLChecker{
		id:   consts.CheckerIDNCCL,
		name: "NCCLTimeoutChecker",
		cfg:  cfg,
	}
}

func (c *NCCLChecker) Name() string {
	return c.name
}

func (c *NCCLChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*NCCLInfo)
	if !ok {
		return nil, fmt.Errorf("wrong input of NCCLChecker")
	}

	podsSet := make(map[string]struct{})
	devicePodNames := make([]string, 0)
	for i := 0; i < len(info.FileName); i++ {
		podName, err := getPodNameFromFileName(info.FileName[i])
		if err != nil {
			logrus.WithError(err).Error("failed to get pod name from fileName")
			continue
		}
		if _, exists := podsSet[podName]; !exists {
			podsSet[podName] = struct{}{}
			devicePodNames = append(devicePodNames, fmt.Sprintf(":%s", podName))
		}
	}

	var raw string
	js, err := info.JSON()
	if err != nil {
		logrus.WithError(err).Error("failed to get info")
	}
	raw = raw + js + "\n"

	status := consts.StatusNormal
	var suggest string
	if len(info.Name) != 0 {
		status = consts.StatusAbnormal
		suggest = "nccl log has timeout error"
	}

	result := config.NCCLCheckItems["NCCLTimeout"]
	result.Device = strings.Join(devicePodNames, ",")
	result.Curr = strconv.Itoa(len(info.Name))
	result.Status = status
	result.Detail = raw
	result.Suggestion = suggest
	return &result, nil

	// return &common.CheckerResult{
	// 	Name:        c.name,
	// 	Description: "Get NCCL timeout error from pod log file",
	// 	Device:      strings.Join(podsName, ","),
	// 	Spec:        "0",
	// 	Curr:        strconv.Itoa(len(info.Name)),
	// 	Status:      status,
	// 	Level:       config.LevelCritical,
	// 	Suggestion:  suggest,
	// 	Detail:      raw,
	// 	ErrorName:   config.ErrorNameNCCL,
	// }, nil
}

func getPodNameFromFileName(fileName string) (string, error) {
	paths := strings.Split(fileName, "/")
	if len(paths) < 5 {
		return "", fmt.Errorf("invalid fileName format=%s, expected at least four '/' character", fileName)
	}
	parts := strings.Split(paths[4], "_")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid fileName format=%s, expected at least one '_' character", fileName)
	}
	return parts[1], nil
}
