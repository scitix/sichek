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
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewInstallCmd() *cobra.Command {
	var (
		imageRepo       string
		imageTag        string
		defaultSpec     string
		namespace       string
		hostfile        string
		host            string
		operatingSystem string
	)

	runCmd := &cobra.Command{
		Use:   "install",
		Short: "Install or Update sichek via Helm",
		Long: `Usage: sichek install [flags]

Defaults:
  --image-repo       = registry-us-east.scitix.ai/hisys/sichek
  --image-tag        = latest
  --default-spec     = "hercules_spec.yaml"
  --namespace        = "default"
  --hostfile         = None (file containing hostnames, one per line)
  --host             = None (comma-separated hostnames)

Note: Number of workers will be automatically derived from hostfile or host parameter.`,
		Run: func(cmd *cobra.Command, args []string) {
			imageRepo = viper.GetString("image_repo")
			imageTag = viper.GetString("image_tag")
			defaultSpec = viper.GetString("default_spec")

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(600)*time.Second)
			defer cancel()

			script := "/var/sichek/scripts/sichek_install.sh"

			argList := []string{
				namespace,
				imageRepo,
				imageTag,
				defaultSpec,
				hostfile,
				host,
				operatingSystem,
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

	runCmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace for sichek")
	runCmd.Flags().StringVar(&hostfile, "hostfile", "None", "File containing hostnames, one per line")
	runCmd.Flags().StringVar(&host, "host", "None", "Comma-separated hostnames")
	runCmd.Flags().StringVar(&operatingSystem, "os", "ubuntu", "Operating system (ubuntu or centos)")

	return runCmd
}
