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
package command

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/cmd/command/specgen"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ConfigKeys = []string{
	"image_repo",
	"image_tag",
	"pytorchjob_image_repo",
	"pytorchjob_image_tag",
	"multinode_modeltest_cmd",
	"singlenode_modeltest_cmd",
	"scheduler",
	"roce_shared_mode",
	"default_spec",
	"swanlab_api_key",
	"swanlab_workspace",
	"swanlab_project",
	"swanlab_experiment_name",
}

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage sichek configuration",
		Long:  "Interactive configuration tool for sichek",
	}

	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigSetCmd())

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Init configuration interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir := filepath.Join(os.Getenv("HOME"), ".sichek")
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config dir: %v", err)
			}
			configFile := filepath.Join(configDir, "config.yaml")

			// read existing config file and override with new values
			v := viper.New()
			v.SetConfigFile(configFile)
			_ = v.ReadInConfig()

			reader := bufio.NewReader(os.Stdin)

			cfg := map[string]string{
				"image_repo":               ask(reader, v, "image_repo", "sichek image repository", "ghcr.io/scitix/sichek"),
				"image_tag":                ask(reader, v, "image_tag", "sichek image tag", "latest"),
				"pytorchjob_image_repo":    ask(reader, v, "pytorchjob_image_repo", "pytorchjob image repository", ""),
				"pytorchjob_image_tag":     ask(reader, v, "pytorchjob_image_tag", "pytorchjob image tag", ""),
				"multinode_modeltest_cmd":  ask(reader, v, "multinode_modeltest_cmd", "multinode modeltest cmd", ""),
				"singlenode_modeltest_cmd": ask(reader, v, "singlenode_modeltest_cmd", "singlenode modeltest cmd", ""),
				"scheduler":                ask(reader, v, "scheduler", "k8s scheduler", ""),
				"roce_shared_mode":         ask(reader, v, "roce_shared_mode", "roce shared mode", "none"),
				"default_spec":             ask(reader, v, "default_spec", "default spec", ""),
				"swanlab_api_key":          ask(reader, v, "swanlab_api_key", "swanlab api key", ""),
				"swanlab_workspace":        ask(reader, v, "swanlab_workspace", "swanlab workspace", ""),
				"swanlab_proj_name":        ask(reader, v, "swanlab_proj_name", "swanlab project", ""),
			}

			// Validate default_spec before saving config
			if defaultSpec, exists := cfg["default_spec"]; exists && defaultSpec != "" {
				if !validateSpecExists(defaultSpec) {
					// Spec validation failed, exit gracefully
					return nil
				}
			}

			// write config file
			for k, vval := range cfg {
				v.Set(k, vval)
			}
			if err := v.WriteConfigAs(configFile); err != nil {
				fmt.Println("❌ Failed to write config: ", err)
				return nil
			}

			fmt.Printf("✅ Config saved to %s\n", configFile)
			return nil
		},
	}
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "View current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := filepath.Join(os.Getenv("HOME"), ".sichek", "config.yaml")
			v := viper.New()
			v.SetConfigFile(configFile)

			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config: %v", err)
			}

			fmt.Println("Current configuration:")
			for _, key := range ConfigKeys {
				val := v.GetString(key)
				fmt.Printf("  %-12s : %s\n", key, val)
			}
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Interactively update configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := filepath.Join(os.Getenv("HOME"), ".sichek", "config.yaml")
			v := viper.New()
			v.SetConfigFile(configFile)
			v.ReadInConfig()

			reader := bufio.NewReader(os.Stdin)

			fmt.Println("Choose the key to modify:")
			for i, key := range ConfigKeys {
				fmt.Printf("  [%d] %s (current: %s)\n", i+1, key, v.GetString(key))
			}

			fmt.Print("\nEnter number to select key: ")
			var idx int
			fmt.Scan(&idx)
			if idx < 1 || idx > len(ConfigKeys) {
				return fmt.Errorf("invalid choice")
			}

			selectedKey := ConfigKeys[idx-1]
			fmt.Printf("Enter new value for '%s': ", selectedKey)
			newVal, _ := reader.ReadString('\n')
			newVal = strings.TrimSpace(newVal)
			if newVal == "" {
				fmt.Println("❌ No value entered, canceled.")
				return nil
			}
			v.Set(selectedKey, newVal)
			v.WriteConfigAs(configFile)
			fmt.Printf("✅ Updated %s = %s\n", selectedKey, newVal)
			return nil
		},
	}
}

func ask(reader *bufio.Reader, v *viper.Viper, key, desc, def string) string {
	current := v.GetString(key)
	if current == "" {
		current = def
	}
	fmt.Printf("%s [%s]: ", desc, current)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return current
	}
	return input
}

// validateSpecExists checks if the specified spec file exists in production path or SICHEK_SPEC_URL
// Returns true if spec exists, false otherwise
func validateSpecExists(specName string) bool {
	specPath, err := specgen.EnsureSpecFile(specName)
	if err == nil {
		fmt.Printf("✅ Found spec file in production path: %s\n", specPath)
		return true
	}
	// Spec not found anywhere
	fmt.Println("❌ Spec validation failed!")
	fmt.Printf("📁 Spec file '%s' not found in:\n", specName)
	fmt.Printf("   - Production path: %s\n", consts.DefaultProductionCfgPath)
	specURL := httpclient.GetSichekSpecURL()
	if specURL != "" {
		fmt.Printf("   - SICHEK_SPEC_URL: %s\n", specURL)
	} else {
		fmt.Printf("   - SICHEK_SPEC_URL: (SICHEK_SPEC_URL environment variable not set)\n")
	}
	fmt.Println()
	fmt.Println("💡 Please create the spec file first:")
	fmt.Println("   sichek spec create -f spec-filename")
	fmt.Println()
	fmt.Println("🔄 Then run the config command again:")
	fmt.Println("   sichek config init")
	fmt.Println()
	return false
}
