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
// Package systemd provides the systemd artifacts and variables for the gpud server.
package systemd

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed sichek.service
var SichekService string

//go:embed sichek.logrotate.conf
var SichekLogrotate string

const (
	DefaultEnvFile       = "/etc/default/sichek"
	DefaultUnitFile      = "/etc/systemd/system/sichek.service"
	DefaultLogrotateConf = "/etc/logrotate.d/sichek"
	DefaultBinPath       = "/usr/local/bin/sichek"
)

func DefaultBinExists() bool {
	_, err := os.Stat(DefaultBinPath)
	return err == nil
}

func CreateDefaultEnvFile() error {
	if _, err := os.Stat(DefaultEnvFile); err == nil { // to not overwrite
		return nil
	}

	f, err := os.OpenFile(DefaultEnvFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Printf("failed to close file %s : %v", DefaultEnvFile, err)
		}
	}(f)

	_, err = f.WriteString(`# sichek environment variables are set here
FLAGS=""
`)
	return err
}

func LogrotateInit() error {
	if _, err := os.Stat(DefaultLogrotateConf); os.IsNotExist(err) {
		return writeConfigFile()
	}
	content, err := os.ReadFile(DefaultLogrotateConf)
	if err != nil {
		return fmt.Errorf("failed to read logrotate config file: %w", err)
	}
	if strings.TrimSpace(string(content)) != strings.TrimSpace(SichekLogrotate) {
		return writeConfigFile()
	}
	return nil
}

func writeConfigFile() error {
	if err := os.WriteFile(DefaultLogrotateConf, []byte(SichekLogrotate), 0644); err != nil {
		return fmt.Errorf("failed to write logrotate config file: %w", err)
	}
	return nil
}
