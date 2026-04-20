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
package config

import (
	"fmt"
	"os"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewSyncCmd creates the "config sync" subcommand that downloads
// spec and user-config files from OSS to the local config directory.
func NewSyncCmd() *cobra.Command {
	var (
		specName string
		cfgName  string
		ossURL   string
	)

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync configuration files from OSS to local",
		Long: `Download the hardware spec and user config files from OSS.

By default, the cluster name is derived from NODE_NAME or hostname,
and files are downloaded from SICHEK_SPEC_URL. Use flags to override.`,
		Run: func(cmd *cobra.Command, args []string) {
			// If --url is provided, set it as SICHEK_SPEC_URL for this invocation
			if ossURL != "" {
				os.Setenv("SICHEK_SPEC_URL", ossURL)
			}

			var hasError bool

			// Sync spec file
			specPath, err := common.EnsureSpecFile(specName, consts.DefaultSpecCfgName)
			if err != nil {
				logrus.WithField("config", "sync").Errorf("failed to sync spec: %v", err)
				hasError = true
			} else {
				fmt.Printf("[config sync] spec: %s\n", specPath)
			}

			// Sync user config file
			cfgPath, err := common.EnsureSpecFile(cfgName, consts.DefaultUserCfgName)
			if err != nil {
				logrus.WithField("config", "sync").Errorf("failed to sync user config: %v", err)
				hasError = true
			} else {
				fmt.Printf("[config sync] user config: %s\n", cfgPath)
			}

			if hasError {
				os.Exit(1)
			}
			fmt.Println("[config sync] done")
		},
	}

	syncCmd.Flags().StringVar(&specName, "spec", "", "spec file name or URL (default: auto-derive from cluster name)")
	syncCmd.Flags().StringVar(&cfgName, "cfg", "", "user config file name or URL (default: auto-derive from cluster name)")
	syncCmd.Flags().StringVar(&ossURL, "url", "", "OSS base URL (overrides SICHEK_SPEC_URL)")

	return syncCmd
}
