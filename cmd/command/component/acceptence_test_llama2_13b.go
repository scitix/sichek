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
	"github.com/spf13/viper"
)

func NewATLlama13bCmd() *cobra.Command {
	var (
		jobName           string
		namespace         string
		cmdStr            string
		imageRepo         string
		imageTag          string
		timeoutToComplete int
		scheduler         string
		roceSharedMode    string
		script            string
		hostfile          string
		host              string
	)

	runCmd := &cobra.Command{
		Use:   "at-llama13b",
		Short: "Run llama13b benchmark via helm install a mpijob and gather metrics",
		Long: `Usage: sichek at-llama13b [flags]

Defaults:
  --job-name         = llama2-13b-bench
  --namespace        = default
  --cmd              = TP=2 PP=1 GBS=256 MBS=1 MAX_STEPS=64 MOCK_DATA=true ENABLE_CKPT=0 LOG_INTERVAL=1 EVAL_INTERVAL=200 bash /workspace/deep_learning_examples/training/Megatron-LM/llm/llama/run_meg_lm_llama2_13b_bf16.sh
  --image-repo       = registry-us-east.scitix.ai/hpc/ngc_pytorch
  --image-tag        = 24.06-sicl-0723
  --timeout          = 600
  --scheduler        = si-scheduler
  --roceSharedMode   = none
  --hostfile         = None (file containing hostnames, one per line)
  --host             = None (comma-separated hostnames)`,
		Run: func(cmd *cobra.Command, args []string) {
			imageRepo = viper.GetString("pytorchjob_image_repo")
			imageTag = viper.GetString("pytorchjob_image_tag")
			cmdStr = viper.GetString("at_llama13b_cmd")
			scheduler = viper.GetString("scheduler")
			roceSharedMode = viper.GetString("roce_shared_mode")

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
				scheduler,
				roceSharedMode,
				hostfile,
				host,
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

	runCmd.Flags().StringVar(&jobName, "job-name", "llama2-13b-bench", "Name of the PyTorchJob")
	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace")
	runCmd.Flags().IntVar(&timeoutToComplete, "timeout", 3600, "Timeout for job completion in seconds")
	runCmd.Flags().StringVar(&script, "script", "/var/sichek/scripts/llama2_13b_benchmark_single_node.sh", "script to run the mpijob")
	runCmd.Flags().StringVar(&hostfile, "hostfile", "None", "File containing hostnames, one per line")
	runCmd.Flags().StringVar(&host, "host", "None", "Comma-separated hostnames")

	return runCmd
}
