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

func NewRoCEGidsCheckCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "gid",
		Short: "Check if all IB ports have correct RoCEv2 GID at fixed indexes (0-3)",
		Run: func(cmd *cobra.Command, args []string) {
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
			}

			ok, issues := checkExpectedGidIndexLayout(verbose)
			if ok {
				fmt.Println("✅ All IB ports have expected GID types at indexes 0-3.")
				ComponentStatuses["roce-gid-layout"] = true
			} else {
				fmt.Println("❌ Found IB ports have unexpected GID types or layout:")
				for _, line := range issues {
					fmt.Println("  -", line)
				}
				ComponentStatuses["roce-gid-layout"] = false
			}
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	return cmd
}

func checkExpectedGidIndexLayout(verbose bool) (bool, []string) {
	var issues []string
	basePath := "/sys/class/infiniband"

	devs, err := os.ReadDir(basePath)
	if err != nil {
		return false, []string{"Cannot read /sys/class/infiniband: " + err.Error()}
	}

	// GID Index → (Expected GID Type, IsIPv4)
	expected := map[string]struct {
		Type   string
		IsIPv4 bool
	}{
		"0": {"IB/RoCE v1", false}, // IPv6
		"1": {"RoCE v2", false},    // IPv6
		"2": {"IB/RoCE v1", true},  // IPv4
		"3": {"RoCE v2", true},     // IPv4
	}

	ipv4MappedGIDPattern := regexp.MustCompile(`^(0000:){5}ffff:`)

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
			// Check all GID indexes under this port
			gidDir := filepath.Join(portPath, portNum, "gids")
			typesDir := filepath.Join(portPath, portNum, "gid_attrs", "types")

			for idx, expect := range expected {
				gidFile := filepath.Join(gidDir, idx)
				typeFile := filepath.Join(typesDir, idx)

				gidValBytes, err1 := os.ReadFile(gidFile)
				typeValBytes, err2 := os.ReadFile(typeFile)

				if err1 != nil || err2 != nil {
					issues = append(issues, fmt.Sprintf(
						"%s port %s: Cannot read GID/type for index %s", devName, portNum, idx))
					continue
				}

				gidVal := strings.TrimSpace(string(gidValBytes))
				typeVal := strings.TrimSpace(string(typeValBytes))

				isIPv4 := ipv4MappedGIDPattern.MatchString(gidVal)

				if typeVal != expect.Type || isIPv4 != expect.IsIPv4 {
					issues = append(issues, fmt.Sprintf(
						"%s port %s: index %s expected [%s, IPv4=%v], got [%s, IPv4=%v]",
						devName, portNum, idx, expect.Type, expect.IsIPv4, typeVal, isIPv4))
				} else if verbose {
					logrus.Infof("%s port %s index %s: OK (%s, IPv4=%v)",
						devName, portNum, idx, typeVal, isIPv4)
				}
			}
		}
	}

	return len(issues) == 0, issues
}
