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

func NewATNCCLTest1Cmd() *cobra.Command {
	var (
		jobName           string
		namespace         string
		nodeSelector      string
		numWorkers        int
		cmdStr            string
		imageRepo         string
		imageTag          string
		timeoutToComplete int
		scheduler         string
		macvlan           bool
	)

	runCmd := &cobra.Command{
		Use:   "at-nccltest1",
		Short: "Run a Mpijob via Helm and gather metrics",
		Long: `Usage: sichek run-job [flags]

Defaults:
  --job-name         = at-nccltest1
  --namespace        = default
  --node-selector    = sichek=test
  --num-workers      = 2
  --cmd              = ""
  --image-repo       = registry-cn-shanghai.siflow.cn/hisys/sichek
  --image-tag        = latest
  --timeout          = 600
	--scheduler        = sischeduler
  --macvlan          = false`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutToComplete)*time.Second)
			defer cancel()
			script := "/var/sichek/scripts/nccl_benchmark_single_node.sh"
			// 构造参数顺序：<job> <namespace> <cmd> <nodeSelector> <imageRepo> <imageTag> <timeout> <rdmaMode>
			argList := []string{
				jobName,
				namespace,
				nodeSelector,
				fmt.Sprintf("%d", numWorkers),
				cmdStr,
				imageRepo,
				imageTag,
				fmt.Sprintf("%d", timeoutToComplete),
				scheduler,
				fmt.Sprintf("%t", macvlan),
			}

			command := exec.CommandContext(ctx, "bash", append([]string{script}, argList...)...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			if err := command.Run(); err != nil {
				logrus.Errorf("PyTorchJob script failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().StringVar(&jobName, "job-name", "at-nccltest1", "Name of the mpijob")
	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace")
	runCmd.Flags().StringVar(&nodeSelector, "node-selector", "sichek=test", "Node selector")
	runCmd.Flags().IntVar(&numWorkers, "num-workers", 2, "Number of worker pods")
	runCmd.Flags().StringVar(&cmdStr, "cmd", "", "Command to run inside pod")
	runCmd.Flags().StringVar(&imageRepo, "image-repo", "registry-cn-shanghai.siflow.cn/hisys/sichek", "Image repository")
	runCmd.Flags().StringVar(&imageTag, "image-tag", "v0.5.5", "Image tag")
	runCmd.Flags().IntVar(&timeoutToComplete, "timeout", 600, "Timeout for job completion in seconds")
	runCmd.Flags().StringVar(&scheduler, "scheduler", "sischeduler", "k8s scheduler name to use for the job, ->[sischeduler, unischeduler]")
	runCmd.Flags().BoolVar(&macvlan, "macvlan", false, "RDMA mode: macvlan-roce or not")

	return runCmd
}
