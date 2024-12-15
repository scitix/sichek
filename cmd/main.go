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
package main

import (
	"fmt"
	"os"

	"github.com/scitix/sichek/cmd/command"
	"github.com/scitix/sichek/cmd/command/component"
	"github.com/scitix/sichek/pkg/utils"
)

func main() {
	root := utils.IsRoot()
	if !root {
		fmt.Println("sichek must be run as root")
		os.Exit(1)
	}
	rootCmd := command.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
	if len(component.ComponentStatuses) != 0 {
		printComponentStatuses()
	}
	if !isAllPassed() {
		os.Exit(-1)
	} else {
		os.Exit(0)
	}
}

func isAllPassed() bool {
	component.StatusMutex.Lock()
	defer component.StatusMutex.Unlock()
	for _, passed := range component.ComponentStatuses {
		if !passed {
			return false
		}
	}
	return true
}

func printComponentStatuses() {
	component.StatusMutex.Lock()
	defer component.StatusMutex.Unlock()
	utils.PrintTitle("Summary", "-")
	for componentItem, status := range component.ComponentStatuses {
		statusStr := fmt.Sprintf("%s%s%s", component.Green, "PASS", component.Reset)
		if !status {
			statusStr = fmt.Sprintf("%s%s%s", component.Red, "FAIL", component.Reset)
		}
		fmt.Printf(" - %s: %s\n", componentItem, statusStr)
	}
}