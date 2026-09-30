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
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var badLinkPattern = regexp.MustCompile(`bad status\s+110.*Connection timed out`)

func NewIBLinkCheckCmd() *cobra.Command {
	var timeoutSec int
	var verbose bool

	ibLinkCmd := &cobra.Command{
		Use:   "iblink",
		Short: "Run iblinkinfo to detect InfiniBand switch issues",
		Run: func(cmd *cobra.Command, args []string) {
			timeout := time.Duration(timeoutSec) * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			}

			output, err := runIbLinkInfo(ctx)
			if err != nil {
				logrus.Errorf("Failed to run iblinkinfo: %v", err)
				return
			}

			badLinks := parseIbLinkInfo(output)
			if len(badLinks) > 0 {
				fmt.Printf("⚠️ Detected %d InfiniBand connection issues:\n", len(badLinks))
				for _, line := range badLinks {
					fmt.Println(" - ", line)
					break
				}
				ComponentStatuses["iblink"] = false
			} else {
				fmt.Println("✅ All InfiniBand links are healthy.")
				ComponentStatuses["iblink"] = true
			}
		},
	}

	ibLinkCmd.Flags().IntVarP(&timeoutSec, "timeout", "t", 10, "Timeout in seconds for iblinkinfo")
	ibLinkCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return ibLinkCmd
}

func runIbLinkInfo(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "iblinkinfo")
	outputBytes, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("iblinkinfo timed out")
	}

	if err != nil {
		return "", fmt.Errorf("iblinkinfo failed: %v\nOutput: %s", err, outputBytes)
	}

	return string(outputBytes), nil
}

func parseIbLinkInfo(output string) []string {
	var issues []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if badLinkPattern.MatchString(line) {
			issues = append(issues, line)
		}
	}
	return issues
}
