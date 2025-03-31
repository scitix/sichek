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
package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nccl/checker"
	"github.com/scitix/sichek/components/nccl/config"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

type NCCLCollector struct {
	name string
	cfg  *config.NCCLUserConfig

	RegexpName []string
	Regexp     []string
}

func NewNCCLCollector(ctx context.Context, cfg common.ComponentUserConfig) (*NCCLCollector, error) {
	config, ok := cfg.(*config.NCCLUserConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for GPFS")
	}
	if len(config.NCCL.CheckerConfigs) == 0 {
		return nil, fmt.Errorf("No NCCL Collector indicate in yaml config")
	}

	var regexpName []string
	var regexp []string
	for _, checkers_cfg := range config.NCCL.CheckerConfigs {
		regexpName = append(regexpName, checkers_cfg.Name)
		regexp = append(regexp, checkers_cfg.Regexp)
	}

	// filter, err := filter.NewFilter(
	// 	regexpName,
	// 	regexp,
	// 	allFiles,
	// 	[][]string{},
	// 	5000,
	// )
	// if err != nil {
	// 	logrus.WithError(err).Error("failed to create filter in NCCLCollector")
	// 	return nil, err
	// }

	return &NCCLCollector{
		name:       "NCCLCollector",
		cfg:        config,
		RegexpName: regexpName,
		Regexp:     regexp,
	}, nil
}

func (c *NCCLCollector) Name() string {
	return c.name
}

func (c *NCCLCollector) GetCfg() common.ComponentUserConfig {
	return c.cfg
}

func (c *NCCLCollector) Collect(ctx context.Context) (common.Info, error) {
	allFiles, err := GetAllFilePaths(c.cfg.NCCL.DirPath)
	if err != nil {
		logrus.WithError(err).Errorf("failed to walkdir in %s", c.cfg.NCCL.DirPath)
		return nil, err
	}

	allFiles = filtLogFiles(allFiles)

	filter, err := filter.NewFilter(
		c.RegexpName,
		c.Regexp,
		allFiles,
		[][]string{},
		5000,
	)
	if err != nil {
		logrus.WithError(err).Error("failed to create filter in NCCLCollector")
		return nil, err
	}
	defer filter.Close()

	filterRes := filter.Check()

	var res checker.NCCLInfo
	res.Time = time.Now()
	for i := 0; i < len(filterRes); i++ {
		res.Name = append(res.Name, filterRes[i].Name)
		res.Regexp = append(res.Regexp, filterRes[i].Regex)
		res.FileName = append(res.FileName, filterRes[i].FileName)
		res.Raw = append(res.Raw, filterRes[i].Line)
	}
	return &res, nil
}

var checkedFiles map[string]struct{}

func filtLogFiles(allFiles []string) []string {
	var res []string
	for i := 0; i < len(allFiles); i++ {
		if strings.HasSuffix(allFiles[i], ".gz") {
			continue
		}
		if strings.HasSuffix(allFiles[i], ".log") {
			res = append(res, allFiles[i])
			continue
		}
		if _, exists := checkedFiles[allFiles[i]]; !exists {
			res = append(res, allFiles[i])
		}
	}

	checkedFiles = make(map[string]struct{})
	for i := 0; i < len(allFiles); i++ {
		checkedFiles[allFiles[i]] = struct{}{}
	}
	return res
}

func GetAllFilePaths(dir string) ([]string, error) {
	var filePaths []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			filePaths = append(filePaths, absPath)
		}
		return nil
	})

	return filePaths, err
}
