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
	"time"

	"github.com/scitix/sichek/components/common"
	DmesgCfg "github.com/scitix/sichek/components/dmesg/config"
	"github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

type DmesgInfo struct {
	Time     time.Time
	Name     []string `json:"regex_name"`
	Regexp   []string `json:"regexpression"`
	FileName []string `json:"file_name"`
	Raw      []string `json:"raw_log"`
}

func (d *DmesgInfo) JSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type DmesgChecker struct {
	id   string
	name string
	cfg  *DmesgCfg.DmesgConfig
}

func NewDmesgChecker(cfg *DmesgCfg.DmesgConfig) common.Checker {
	return &DmesgChecker{
		id:   config.CheckerIDDmesg,
		name: "DmesgErrorChecker",
		cfg:  cfg,
	}
}

func (c *DmesgChecker) Name() string {
	return c.name
}

func (c *DmesgChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*DmesgInfo)
	if !ok {
		return nil, fmt.Errorf("wrong input of DmesgChecker")
	}

	var raw string
	js, err := info.JSON()
	if err != nil {
		logrus.WithError(err).Error("failed to get info")
	}
	raw = raw + js + "\n"

	status := config.StatusNormal
	var suggest string
	if len(info.Name) != 0 {
		status = config.StatusAbnormal
		suggest = "check dmesg error"
	}
	return &common.CheckerResult{
		Name:        c.name,
		Description: "Get errors from dmesg",
		Device:      "Dmesgcmd",
		Spec:        "0",
		Curr:        strconv.Itoa(len(info.Name)),
		Status:      status,
		Level:       config.LevelCritical,
		Suggestion:  suggest,
		Detail:      raw,
		ErrorName:   config.ErrorNameDmesg,
	}, nil
}
