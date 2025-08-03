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
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewRunScriptsCmd() *cobra.Command {

	var (
		list              bool
		timeoutToComplete int
	)

	var runCmd = &cobra.Command{
		Use:   "script <script-name> [args...]",
		Short: "Run scripts from /var/sichek/scripts",
		Args:  cobra.ArbitraryArgs, // 支持0个或多个参数，为了兼容 --list
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutToComplete)*time.Second)
			defer cancel()
			scriptDir := "/var/sichek/scripts"

			if list {
				files, err := os.ReadDir(scriptDir)
				if err != nil {
					logrus.Fatalf("Failed to read script directory: %v", err)
				}
				fmt.Println("Available scripts:")
				for _, file := range files {
					if file.IsDir() {
						continue
					}
					info, err := file.Info()
					if err != nil {
						logrus.Warnf("Failed to get info for file %s: %v", file.Name(), err)
						continue
					}
					// 检查是否具有可执行权限（对所有人）
					if info.Mode()&0111 != 0 {
						fmt.Println("  ", file.Name())
					}
				}
				return
			}

			if len(args) < 1 {
				fmt.Fprintln(os.Stderr, "Script name required. Or use --list to view available scripts.")
				cmd.Usage()
				os.Exit(1)
			}

			scriptName := args[0]
			scriptPath := filepath.Join(scriptDir, scriptName)
			scriptArgs := args[1:]

			command := exec.CommandContext(ctx, scriptPath, scriptArgs...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin

			if err := command.Run(); err != nil {
				logrus.Fatalf("Script execution failed: %v", err)
				os.Exit(1)
			}
		},
	}

	runCmd.Flags().BoolVar(&list, "list", false, "List available scripts")
	runCmd.Flags().IntVar(&timeoutToComplete, "timeout", 60, "Timeout for job completion in seconds")
	return runCmd
}
