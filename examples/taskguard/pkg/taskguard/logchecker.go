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
	"context"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"sigs.k8s.io/yaml"
)

type LogCheckerRules struct {
	Errors []string
}

func (c *Controller) checkPodLogs(logs string) bool {
	rulesFile, err := os.ReadFile(c.config.LogCheckerRulesPath)
	if err != nil {
		// skip check
		logx.Errorf("failed to read log checker rules, err: %s", err.Error())
		return false
	}

	var rules LogCheckerRules
	err = yaml.Unmarshal(rulesFile, &rules)
	if err != nil {
		// skip check
		logx.Errorf("failed to unmarshal log checker rules, err: %s", err.Error())
		return false
	}

	for _, rule := range rules.Errors {
		if strings.Contains(logs, rule) {
			return true
		}
	}
	return false
}

func (c *Controller) isTaskPodHangFromLog(ctx context.Context, namespace, podName string) bool {
	// check stdout/stderr logs failed
	logs, err := c.k8sClient.GetPodLogs(ctx, namespace, podName, "", c.config.LogCheckerLines)
	if err != nil {
		logx.Errorf("failed to get pod logs, err: %s", err.Error())
		return false
	}
	return c.checkPodLogs(logs)
}
