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
package taskguard

import (
	"encoding/json"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// sichek level
	SiChekLevelWarning  = "warning"
	SiChekLevelCritical = "critical"
	SiChekLevelFatal    = "fatal"
)

type SiChekResult struct {
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

type annotation struct {
	ErrorName string `json:"error_name"`
	Device    string `json:"device"`
}

func (c *Controller) getSiChekResultFromNode(nodeName string) SiChekResult {
	var res SiChekResult

	node, err := c.node.GetNodeByName(nodeName)
	if err != nil {
		logx.Errorf("failed to get node %s info, err: %s", nodeName, err.Error())
		return res
	}

	siChekResultStr, ok := node.Annotations[c.config.SiChekNodeAnnotationKey]
	if !ok {
		return res
	}

	err = json.Unmarshal([]byte(siChekResultStr), &res)
	if err != nil {
		logx.Errorf("failed to unmarshal sichek result, err: %s", err.Error())
		return res
	}

	return res
}

func (c *Controller) isTaskPodHealthy(nodeName, podName string) bool {
	siChekRes := c.getSiChekResultFromNode(nodeName)

	chekSlice := []map[string][]*annotation{
		siChekRes.NCCL,
		siChekRes.Hang,
		siChekRes.NVIDIA,
		siChekRes.Infiniband,
		siChekRes.Ethernet,
		siChekRes.GPFS,
		siChekRes.CPU,
		siChekRes.Memory,
		siChekRes.Dmesg,
	}

	for _, item := range chekSlice {
		if item == nil {
			continue
		}
		for level, annotations := range item {
			if level != SiChekLevelFatal && level != SiChekLevelCritical {
				continue
			}
			for _, annotation := range annotations {
				logx.Infof("pod %s unhealthy from sichek, annotation %+v", podName, annotation)
				return false
			}
		}
	}

	return true
}

func (c *Controller) isTaskPodHangFromSiChek(nodeName, podName string) bool {
	siChekRes := c.getSiChekResultFromNode(nodeName)

	if siChekRes.Hang == nil {
		return false
	}

	for level, annotations := range siChekRes.Hang {
		if level != SiChekLevelFatal && level != SiChekLevelCritical {
			continue
		}
		for _, annotation := range annotations {
			logx.Infof("pod %s hang from sichek, annotation %+v", podName, annotation)
			return true
		}
	}

	return false
}
