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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewRoCEGidEqualCheckCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "gid-equal",
		Short: "Check if all IB ports have the same IPv4 RoCEv2 GID (usually index 3), for container environments",
		Run: func(cmd *cobra.Command, args []string) {
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			}
			ok, details := checkIPv4RoCEv2GidIndexEqual(verbose)
			if ok {
				fmt.Println("✅ All IB ports have the same IPv4 RoCEv2 GID.")
				ComponentStatuses["rocev2-gid-equal"] = true
			} else {
				fmt.Println("❌ Detected inconsistency in IPv4 RoCEv2 GIDs across IB ports:")
				for _, d := range details {
					fmt.Println("  -", d)
				}
				ComponentStatuses["rocev2-gid-equal"] = false
			}
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	return cmd
}

func checkIPv4RoCEv2GidIndexEqual(verbose bool) (bool, []string) {
	var issues []string
	basePath := "/sys/class/infiniband"
	ipv4MappedGIDPattern := regexp.MustCompile(`^(0000:){5}ffff:`)

	var refIndex string
	var firstFound bool = false

	devs, err := os.ReadDir(basePath)
	if err != nil {
		return false, []string{"Cannot read /sys/class/infiniband: " + err.Error()}
	}

	for _, dev := range devs {
		devName := dev.Name()
		portPath := filepath.Join(basePath, devName, "ports")
		ports, err := os.ReadDir(portPath)
		if err != nil {
			continue
		}

		for _, port := range ports {
			portNum := port.Name()
			gidDir := filepath.Join(portPath, portNum, "gids")
			gids, err := os.ReadDir(gidDir)
			if err != nil {
				issues = append(issues, fmt.Sprintf("%s port %s: cannot read GID directory", devName, portNum))
				continue
			}

			found := false

			for _, gidEntry := range gids {
				idx := gidEntry.Name()
				gidFile := filepath.Join(gidDir, idx)
				typeFile := filepath.Join(portPath, portNum, "gid_attrs", "types", idx)

				gidValBytes, err1 := os.ReadFile(gidFile)
				typeValBytes, err2 := os.ReadFile(typeFile)
				if err1 != nil || err2 != nil {
					continue
				}

				gidVal := strings.TrimSpace(string(gidValBytes))
				typeVal := strings.TrimSpace(string(typeValBytes))

				if typeVal == "RoCE v2" && ipv4MappedGIDPattern.MatchString(gidVal) {
					if !firstFound {
						refIndex = idx
						firstFound = true
						found = true
						if verbose {
							logrus.Infof("Reference GID index is %s (device %s port %s)", refIndex, devName, portNum)
						}
					} else {
						if idx != refIndex {
							issues = append(issues, fmt.Sprintf(
								"%s port %s: IPv4 RoCEv2 GID at index %s, expected %s",
								devName, portNum, idx, refIndex,
							))
						}
						found = true
					}
					break
				}
			}

			if !found {
				issues = append(issues, fmt.Sprintf("%s port %s: No IPv4 RoCEv2 GID found", devName, portNum))
			}
		}
	}

	return len(issues) == 0, issues
}
