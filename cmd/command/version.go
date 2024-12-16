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
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// NewInfinibandCmd创建并返回用于代表Infiniband相关操作的子命令实例，配置命令的基本属性
func NewVersionCmd() *cobra.Command {
	infinibandCmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Print the version number of sichek",
		Long:    "All software has versions. This is sichek's",
		Run: func(cmd *cobra.Command, args []string) {
			version := "v0.1.0"
			gitCommit := getGitCommit()
			goVersion := getGoVersion()
			cmd.Printf("Version: %s\nGit Commit: %s\nGo Version: %s\n", version, gitCommit, goVersion)
		},
	}
	return infinibandCmd
}

// func hasGitInstalled() bool {
// 	_, err := exec.LookPath("git")
// 	return err == nil
// }

func getGitCommitWithShell() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to get HEAD by `git rev-parse HEAD`: %v", err)
	}
	return strings.TrimSpace(string(output))
}

// func getGitCommitWithModule() string {
// 	repo, err := git.PlainOpen(".")
// 	if err != nil {
// 		log.Fatalf("Failed to open Git repository: %v", err)
// 	}

// 	head, err := repo.Head()
// 	if err != nil {
// 		log.Fatalf("Failed to get HEAD: %v", err)
// 	}

// 	return head.Hash().String()
// }

func getGitCommit() string {
	return getGitCommitWithShell()
}

func getGoVersion() string {
	return runtime.Version()
}
