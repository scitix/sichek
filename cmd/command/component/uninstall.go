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
package component

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewUninstallCmd() *cobra.Command {
	var (
		nodeSelector string
		numWorkers   int
		imageRepo    string
		imageTag     string
		namespace    string
	)

	runCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall sichek via Helm",
		Long: `Usage: sichek uninstall [flags]

Defaults:
  --node-selector    = ""
  --num-workers      = 2
  --image-repo       = registry-cn-shanghai.siflow.cn/hisys/sichek
  --image-tag        = v0.5.5`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(600)*time.Second)
			defer cancel()

			argList := []string{
				"upgrade", "--install", "uninstall-all", "/var/sichek/k8s/sichek/",
				"--atomic",
				"--set", "mode=uninstall-all",
				"--set", fmt.Sprintf("image.repository=%s", imageRepo),
				"--set", fmt.Sprintf("image.tag=%s", imageTag),
				"--set", fmt.Sprintf("batchjob.parallelism=%d", numWorkers),
				"--set", fmt.Sprintf("batchjob.completions=%d", numWorkers),
				"--set", fmt.Sprintf("namespace=%s", namespace),
			}
			if nodeSelector != "" {
				escapedNodeSelector := strings.ReplaceAll(nodeSelector, ".", `\.`)
				argList = append(argList, "--set", fmt.Sprintf("nodeSelector.%s", escapedNodeSelector))
			}

			command := exec.CommandContext(ctx, "helm", argList...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			if err := command.Run(); err != nil {
				logrus.Errorf("Install/Update sichek script failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().StringVar(&nodeSelector, "node-selector", "", "Node selector")
	runCmd.Flags().IntVar(&numWorkers, "num-workers", 2, "Number of worker pods")
	runCmd.Flags().StringVar(&imageRepo, "image-repo", "registry-cn-shanghai.siflow.cn/hisys/sichek", "Image repository")
	runCmd.Flags().StringVar(&imageTag, "image-tag", "v0.5.5", "Image tag")
	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace for sichek")
	return runCmd
}
