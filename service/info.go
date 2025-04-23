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
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/metrics"

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

func (a *nodeAnnotation) getAnnotationsByItem(item string) (map[string][]*annotation, error) {
	if item == "" {
		return nil, fmt.Errorf("input item is empty")
	}
	switch item {
	case consts.ComponentNameCPU:
		return a.CPU, nil
	case consts.ComponentNameDmesg:
		return a.Dmesg, nil
	case consts.ComponentNameEthernet:
		return a.Ethernet, nil
	case consts.ComponentNameGpfs:
		return a.GPFS, nil
	case consts.ComponentNameHang:
		return a.Hang, nil
	case consts.ComponentNameInfiniband:
		return a.Infiniband, nil
	case consts.ComponentNameNCCL:
		return a.NCCL, nil
	case consts.ComponentNameNvidia:
		return a.NVIDIA, nil
	}
	return nil, fmt.Errorf("input item %s is not supported", item)
}

func (a *nodeAnnotation) setAnnotationsByItem(item string, annotations map[string][]*annotation) error {
	if item == "" {
		return fmt.Errorf("input item is empty")
	}
	switch item {
	case consts.ComponentNameCPU:
		a.CPU = annotations
	case consts.ComponentNameDmesg:
		a.Dmesg = annotations
	case consts.ComponentNameEthernet:
		a.Ethernet = annotations
	case consts.ComponentNameGpfs:
		a.GPFS = annotations
	case consts.ComponentNameHang:
		a.Hang = annotations
	case consts.ComponentNameInfiniband:
		a.Infiniband = annotations
	case consts.ComponentNameNCCL:
		a.NCCL = annotations
	case consts.ComponentNameNvidia:
		a.NVIDIA = annotations
	}
	return nil
}

func (a *nodeAnnotation) updateAnnotations(annotations map[string][]*annotation, result *common.Result) error {
	if result == nil {
		return fmt.Errorf("input result is empty")
	}
	preAnnoStr, err := a.JSON()
	if err != nil {
		return fmt.Errorf("error marshaling pre_anno_str: %v", err)
	}
	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}

	existAnnotation := make(map[string]*annotation)
	for _, annotation := range annotations {
		for _, item := range annotation {
			existAnnotation[item.ErrorName] = item
		}
	}

	newAnnotation := make(map[string][]*annotation)
	if result.Status == consts.StatusAbnormal {
		for _, checkResult := range result.Checkers {
			if checkResult.Status == consts.StatusAbnormal {
				anno := &annotation{
					ErrorName: checkResult.ErrorName,
					Devcie:    checkResult.Device,
				}
				_, exist := newAnnotation[checkResult.Level]
				if !exist {
					newAnnotation[checkResult.Level] = make([]*annotation, 0)
				} else {
					annoNow, exists := existAnnotation[anno.ErrorName]
					if !exists {
						newAnnotation[checkResult.Level] = append(newAnnotation[checkResult.Level], anno)
					} else {
						annoNow.Devcie = checkResult.Device
						newAnnotation[checkResult.Level] = append(newAnnotation[checkResult.Level], annoNow)
					}
				}
			}
		}
	}

	err = a.setAnnotationsByItem(result.Item, newAnnotation)
	if err != nil {
		return fmt.Errorf("error setting annotations by item %v: %v", result.Item, err)
	}
	annoStr, err := a.JSON()
	if err != nil {
		return fmt.Errorf("error marshaling updated annotation: %v", err)
	}
	m := metrics.GetHealthCheckResMetrics()
	m.ExportAnnotationMetrics(annoStr)
	if result.Status == consts.StatusAbnormal && (result.Level == consts.LevelCritical || result.Level == consts.LevelFatal) {
		logrus.Infof("set node annotation for check result %s", jsonData)
		logrus.Infof("update node annotataion from %s to %s", preAnnoStr, annoStr)
	}

	return nil

}

func (a *nodeAnnotation) ParseFromResult(result *common.Result) error {
	annotations := make(map[string][]*annotation)
	err := a.updateAnnotations(annotations, result)
	if err != nil {
		return fmt.Errorf("error updating annotations: %v", err)
	}
	return nil
}

func (a *nodeAnnotation) AppendFromResult(result *common.Result) error {
	annotations, err := a.getAnnotationsByItem(result.Item)
	if err != nil {
		return fmt.Errorf("error getting annotations by item %v: %v", result.Item, err)
	}
	err = a.updateAnnotations(annotations, result)
	if err != nil {
		return fmt.Errorf("error updating annotations: %v", err)
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
