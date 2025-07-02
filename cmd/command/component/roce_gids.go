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
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewRoCEV2GidsCheckCmd() *cobra.Command {
	var verbose bool
	var gidIndex string

	cmd := &cobra.Command{
		Use:   "gid",
		Short: "Check if all IB ports have correct RoCEv2 GID index 3 with IPv4",
		Run: func(cmd *cobra.Command, args []string) {
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			}

			ok, details := checkRoCEv2GidIndex3(gidIndex, verbose)
			if ok {
				fmt.Printf("✅ All IB ports have IPv4 RoCEv2 GID at index %s.\n", gidIndex)
				ComponentStatuses["rocev2-gid"] = true
			} else {
				fmt.Printf("❌ Found IB ports missing IPv4 RoCEv2 GID at index %s:\n", gidIndex)
				for _, line := range details {
					fmt.Println("  -", line)
				}
				ComponentStatuses["rocev2-gid"] = false
			}
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	cmd.Flags().StringVarP(&gidIndex, "gid-index", "x", "3", "GID index checked")
	return cmd
}

func checkRoCEv2GidIndex3(gidIndex string, verbose bool) (bool, []string) {
	var issues []string
	basePath := "/sys/class/infiniband"

	devs, err := os.ReadDir(basePath)
	if err != nil {
		return false, []string{"Cannot read /sys/class/infiniband: " + err.Error()}
	}

	for _, dev := range devs {
		devName := dev.Name()
		vfPath := path.Join(basePath, devName, "device", "physfn")
		if _, err := os.Stat(vfPath); err == nil {
			continue // Skip virtual functions
		}
		portPath := filepath.Join(basePath, devName, "ports")
		ports, err := os.ReadDir(portPath)
		if err != nil {
			continue
		}

		for _, port := range ports {
			portNum := port.Name()
			found := false
			actualIPv4RoCEv2Indexes := []string{}

			// Check all GID indexes under this port
			gidDir := filepath.Join(portPath, portNum, "gids")
			gids, err := os.ReadDir(gidDir)
			if err != nil {
				continue
			}

			for _, gidEntry := range gids {
				idx := gidEntry.Name()
				gidFile := filepath.Join(gidDir, idx)
				typeFile := filepath.Join(portPath, portNum, "gid_attrs", "types", idx)

				gidValBytes, err1 := os.ReadFile(gidFile)
				typeValBytes, err2 := os.ReadFile(typeFile)
				// if verbose {
				// 	logrus.Infof("Checking device %s/%s/gids/%s: gid=%v, type=%v", devName, portNum, idx, gidValBytes, typeValBytes)
				// }
				if err1 != nil || err2 != nil {
					continue
				}

				gidVal := strings.TrimSpace(string(gidValBytes))
				typeVal := strings.TrimSpace(string(typeValBytes))
				// if verbose {
				// 	logrus.Infof("Checking device %s/%s/gids/%s: gid=%s, type=%s", devName, port, idx, gidVal, typeVal)
				// }
				var ipv4MappedGIDPattern = regexp.MustCompile(`^(0000:){5}ffff:`)
				if typeVal == "RoCE v2" && ipv4MappedGIDPattern.MatchString(gidVal) {
					if idx == gidIndex {
						if verbose {
							logrus.Infof("Found IPv4 RoCEv2 GID at index %s for device %s port %s", gidIndex, devName, portNum)
						}
						// Found the expected GID at index 3
						found = true
						break
					} else {
						actualIPv4RoCEv2Indexes = append(actualIPv4RoCEv2Indexes, idx)
					}
				}
			}

			if !found {
				if len(actualIPv4RoCEv2Indexes) > 0 {
					issues = append(issues, fmt.Sprintf(
						"%s port %s: IPv4 RoCEv2 GID found at index(es): %s, but not at index %s",
						devName, portNum, strings.Join(actualIPv4RoCEv2Indexes, ", "), gidIndex,
					))
				} else {
					issues = append(issues, fmt.Sprintf(
						"%s port %s: No IPv4 RoCEv2 GID found",
						devName, portNum,
					))
				}
			}
		}
	}

	return len(issues) == 0, issues
}
