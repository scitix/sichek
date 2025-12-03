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
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	Major     = ""
	Minor     = ""
	Patch     = ""
	GitCommit = "none"
	GoVersion = "none"
	BuildTime = "unknown"
)

func NewVersionCmd() *cobra.Command {
	VersionCmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Print the version number of sichek",
		Long:    "All software has versions. This is sichek's",
		Run: func(cmd *cobra.Command, args []string) {
			var version string
			if GitCommit == "none" {
				GitCommit = getGitCommit()
			}
			if GoVersion == "none" {
				GoVersion = getGoVersion()
			}
			if BuildTime == "unknown" {
				now := time.Now()
				BuildTime = now.Format("2006-01-02T15:04:05")
			}
			if Major == "" {
				version = "dev-" + GitCommit
			} else {
				version = "v" + Major + "." + Minor + "." + Patch
			}
			cmd.Printf("Version: %s\nGit Commit: %s\nGo Version: %s\nBuildTime: %s\n", version, GitCommit, GoVersion, BuildTime)
		},
	}
	return VersionCmd
}

func getGitCommitWithShell() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Failed to get HEAD by `git rev-parse HEAD`: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func getGitCommit() string {
	return getGitCommitWithShell()
}

func getGoVersion() string {
	return runtime.Version()
}
