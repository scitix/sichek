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

func NewPytorchjobCmd() *cobra.Command {
	var (
		jobName           string
		namespace         string
		cmdStr            string
		imageRepo         string
		imageTag          string
		timeoutToComplete int
		rdmaMode          string
		script            string
	)

	runCmd := &cobra.Command{
		Use:   "pytorch",
		Short: "Run a PyTorchJob via Helm and gather TFLOPS metrics",
		Long: `Usage: sichek run-job [flags]

Defaults:
  --job-name         = llama2-70b-bench
  --namespace        = default
  --cmd              = MAX_STEPS=4 EVAL_ITERS=1 MOCK_DATA=true LOG_INTERVAL=1 bash /workspace/Megatron-LM/examples/llama/train_llama2_70b_bf16.sh
  --image-repo       = registry-us-east.scitix.ai/hpc/ngc_pytorch
  --image-tag        = 24.06-sicl-0723
  --timeout          = 600
  --rdma_mode        = pytorchjob-ib`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutToComplete)*time.Second)
			defer cancel()

			// 构造参数顺序：<job> <namespace> <cmd> <nodeSelector> <imageRepo> <imageTag> <timeout> <rdmaMode>
			argList := []string{
				jobName,
				namespace,
				cmdStr,
				imageRepo,
				imageTag,
				fmt.Sprintf("%d", timeoutToComplete),
				rdmaMode,
			}

			command := exec.CommandContext(ctx, script, argList...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			if err := command.Run(); err != nil {
				logrus.Errorf("PyTorchJob script failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().StringVar(&jobName, "job-name", "llama2-70b-bench", "Name of the PyTorchJob")
	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace")
	runCmd.Flags().StringVar(&cmdStr, "cmd", "MAX_STEPS=4 EVAL_ITERS=1 MOCK_DATA=true LOG_INTERVAL=1 bash /workspace/Megatron-LM/examples/llama/train_llama2_70b_bf16.sh", "Command to run inside pod")
	runCmd.Flags().StringVar(&imageRepo, "image-repo", "registry-us-east.scitix.ai/hpc/ngc_pytorch", "Image repository")
	runCmd.Flags().StringVar(&imageTag, "image-tag", "24.06-sicl-0723", "Image tag")
	runCmd.Flags().IntVar(&timeoutToComplete, "timeout", 600, "Timeout for job completion in seconds")
	runCmd.Flags().StringVar(&rdmaMode, "rdma_mode", "pytorchjob-ib", "RDMA mode: pytorchjob-ib or pytorchjob-macvlan-roce")
	runCmd.Flags().StringVar(&script, "script", "/var/sichek/scripts/llama2_70b_benchmark_multi_node.sh", "script to run the PyTorchJob")

	return runCmd
}
