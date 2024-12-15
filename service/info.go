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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

type nodeAnnotation struct {
	NCCL       map[string][]*annotation `json:"nccl"`
	Hang       map[string][]*annotation `json:"hang"`
	NVIDIA     map[string][]*annotation `json:"nvidia"`
	Infiniband map[string][]*annotation `json:"infiniband"`
	Ethernet   map[string][]*annotation `json:"ethernet"`
	GPFS       map[string][]*annotation `json:"gpfs"`
	CPU        map[string][]*annotation `json:"cpu"`
	Memory     map[string][]*annotation `json:"memory"`
	Dmesg      map[string][]*annotation `json:"dmesg"`
}

func GetAnnotationFromJson(jsonStr string) (*nodeAnnotation, error) {
	var anno nodeAnnotation
	if len(jsonStr) == 0 {
		return &anno, nil
	}
	err := json.Unmarshal([]byte(jsonStr), &anno)
	if err != nil {
		return nil, err
	}
	return &anno, nil
}

func (a *nodeAnnotation) JSON() (string, error) {
	data, err := json.Marshal(a)
	return string(data), err
}

func (a *nodeAnnotation) ParseFromResult(result *common.Result) error {
	if result == nil {
		return fmt.Errorf("input result is empty")
	}
	jsonData, err := json.Marshal(result)
	if err != nil {
		logrus.Errorf("Error marshaling JSON: %v", err)
		return err
	}
	pre_anno_str, err := a.JSON()
	if err != nil {
		logrus.Errorf("Error marshaling annotation: %v", err)
		return err
	}
	var annotations map[string][]*annotation
	if result.Status == config.StatusAbnormal {
		annotations = make(map[string][]*annotation)
		for _, check_result := range result.Checkers {
			if check_result.Status == config.StatusAbnormal {
				anno := &annotation{
					ErrorName: check_result.ErrorName,
					Devcie:    check_result.Device,
				}
				_, exist := annotations[check_result.Level]
				if !exist {
					annotations[check_result.Level] = make([]*annotation, 0)
				}
				annotations[check_result.Level] = append(annotations[check_result.Level], anno)
			}
		}
	}
	switch result.Item {
	case config.ComponentNameCPU:
		a.CPU = annotations
	case config.ComponentNameDmesg:
		a.Dmesg = annotations
	case config.ComponentNameEthernet:
		a.Ethernet = annotations
	case config.ComponentNameGpfs:
		a.GPFS = annotations
	case config.ComponentNameHang:
		a.Hang = annotations
	case config.ComponentNameInfiniband:
		a.Infiniband = annotations
	case config.ComponentNameNCCL:
		a.NCCL = annotations
	case config.ComponentNameNvidia:
		a.NVIDIA = annotations
	}
	annoStr, err := a.JSON()
	if err != nil {
		logrus.Errorf("Error marshaling annotation: %v", err)
		return err
	}
	if result.Status == config.StatusAbnormal && (result.Level == config.LevelCritical || result.Level == config.LevelFatal) {
		logrus.Infof("set node annotation for check result %s", jsonData)
		logrus.Infof("update node annotataion from %s to %s", pre_anno_str, annoStr)
	}

	return nil
}

type annotation struct {
	ErrorName string `json:"error_name"`
	Devcie    string `json:"device"`
}

func (a *annotation) JSON() (string, error) {
	data, err := json.Marshal(a)
	return string(data), err
}
