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

func NewDeployCmd() *cobra.Command {
	var (
		imageRepo   string
		imageTag    string
		gpuLabel    string
		deployToCPU bool
		defaultSpec string
		namespace   string
	)

	runCmd := &cobra.Command{
		Use:   "deploy",
		Short: "deploy sichek daemon via Helm",
		Long: `Usage: sichek deploy [flags]

Defaults:
  --image-repo       = registry-us-east.scitix.ai/hisys/sichek
  --image-tag        = latest,
	--gpu-label        = ""
	--cpu    = false
	--default-spec     = "hercules_spec.yaml"
	--namespace        = "hi-sys-monitor"`,
		Run: func(cmd *cobra.Command, args []string) {
			imageRepo = viper.GetString("image_repo")
			imageTag = viper.GetString("image_tag")
			defaultSpec = viper.GetString("default_spec")

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(600)*time.Second)
			defer cancel()
			argList := []string{
				"upgrade", "--install", "sichek-daemon", "/var/sichek/k8s/sichek/",
				"--atomic",
				"--set", "mode=daemon",
				"--set", fmt.Sprintf("image.repository=%s", imageRepo),
				"--set", fmt.Sprintf("image.tag=%s", imageTag),
				"--set", fmt.Sprintf("defaultSpec=%s", defaultSpec),
				"--set", fmt.Sprintf("namespace=%s", namespace),
			}
			if gpuLabel != "" {
				argList = append(argList, "--set", fmt.Sprintf("daemon.gpuLabel=%s", gpuLabel))
			}
			if deployToCPU {
				argList = append(argList, "--set", "daemon.cpu=true")
			}
			fmt.Println("Running command:", "helm", argList)
			command := exec.CommandContext(ctx, "helm", argList...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			if err := command.Run(); err != nil {
				logrus.Errorf("Install/Update sichek script failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().StringVar(&gpuLabel, "gpu-label", "", "gpu label for daemonset pod affinity")
	runCmd.Flags().BoolVar(&deployToCPU, "cpu", false, "deploy sichek to CPU nodes")
	runCmd.Flags().StringVar(&namespace, "namespace", "hi-sys-monitor", "Kubernetes namespace for sichek")
	return runCmd
}
