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

func NewNCCLDiagCmd() *cobra.Command {
	var (
		jobName           string
		namespace         string
		cmdStr            string
		imageRepo         string
		imageTag          string
		timeoutToComplete int
		scheduler         string
		roceSharedMode    string
		hostfile          string
		host              string
		diagMode          string
	)

	runCmd := &cobra.Command{
		Use:   "nccl-diag",
		Short: "Run a NCCL diagnostics via Helm and gather metrics",
		Long: `Usage: sichek nccl-diag [flags]

Defaults:
  --job-name         = nccl-diag
  --namespace        = default
  --cmd              = "bash /var/sichek/scripts/nccltest-diag-bisect.sh"
  --image-repo       = registry-us-east.scitix.ai/hisys/sichek
  --image-tag        = latest
  --timeout          = 600
  --scheduler        = si-scheduler
  --roceSharedMode   = none
  --hostfile         = None (file containing hostnames, one per line)
  --host             = None (comma-separated hostnames)
  --diag-mode        = bisect (bisect mode)`,
		Run: func(cmd *cobra.Command, args []string) {
			imageRepo = viper.GetString("image_repo")
			imageTag = viper.GetString("image_tag")
			scheduler = viper.GetString("scheduler")
			roceSharedMode = viper.GetString("roce_shared_mode")

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutToComplete)*time.Second)
			defer cancel()
			script := "/var/sichek/scripts/nccltest-diag.sh"
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
				diagMode,
			}

			command := exec.CommandContext(ctx, "bash", append([]string{script}, argList...)...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			if err := command.Run(); err != nil {
				logrus.Errorf("NCCL diat script failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().StringVar(&jobName, "job-name", "nccl-diag", "Name of the PyTorchJob")
	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace")
	runCmd.Flags().StringVar(&cmdStr, "cmd", "bash /var/sichek/scripts/nccltest-diag-bisect.sh", "Command to run inside pod")
	runCmd.Flags().IntVar(&timeoutToComplete, "timeout", 600, "Timeout for job completion in seconds")
	runCmd.Flags().StringVar(&hostfile, "hostfile", "None", "File containing hostnames, one per line")
	runCmd.Flags().StringVar(&host, "host", "None", "Comma-separated hostnames")
	runCmd.Flags().StringVar(&diagMode, "diag-mode", "conn", "Bisect mode: conn, perf")

	return runCmd
}
