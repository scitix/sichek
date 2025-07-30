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
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewDiagCmd() *cobra.Command {
	var (
		jobName           string
		namespace         string
		nodeSelector      string
		cmdStr            string
		numWorkers        int
		imageRepo         string
		imageTag          string
		defaultSpec       string
		timeoutToComplete int
	)

	runCmd := &cobra.Command{
		Use:   "diag",
		Short: "run sichek on diag node via Helm",
		Long: `Usage: sichek install [flags]

Defaults:
  --node-selector    = "None"
  --num-workers      = 2
  --image-repo       = registry-cn-shanghai.siflow.cn/hisys/sichek
  --image-tag        = v0.5.4,
	--default-spec     = "hercules_spec.yaml"`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutToComplete)*time.Second)
			defer cancel()
			script := "/var/sichek/scripts/sichek_diag_and_collect_err.sh"

			argList := []string{
				jobName,
				namespace,
				nodeSelector,
				fmt.Sprintf("%d", numWorkers),
				cmdStr,
				imageRepo,
				imageTag,
				defaultSpec,
				fmt.Sprintf("%d", timeoutToComplete),
			}

			command := exec.CommandContext(ctx, "bash", append([]string{script}, argList...)...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			if err := command.Run(); err != nil {
				logrus.Errorf("Install/Update sichek script failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().StringVar(&jobName, "job-name", "diag", "Name of the PyTorchJob")
	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace")
	runCmd.Flags().StringVar(&nodeSelector, "node-selector", "None", "Node selector")
	runCmd.Flags().IntVar(&numWorkers, "num-workers", 2, "Number of worker pods")
	runCmd.Flags().StringVar(&cmdStr, "cmd", "sichek all -e -I podlog,gpuevents,nccltest", "Command to run inside pod")
	runCmd.Flags().StringVar(&imageRepo, "image-repo", "registry-cn-shanghai.siflow.cn/hisys/sichek", "Image repository")
	runCmd.Flags().StringVar(&imageTag, "image-tag", "v0.5.4", "Image tag")
	runCmd.Flags().StringVar(&defaultSpec, "default-spec", "hercules_spec.yaml", "Default spec file for installation")
	runCmd.Flags().IntVar(&timeoutToComplete, "timeout", 1200, "Timeout for job completion in seconds")

	return runCmd
}
