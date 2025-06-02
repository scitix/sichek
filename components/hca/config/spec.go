package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

// HCASpecs holds the specifications for HCA devices.
// There are may be multiple HCA devices on one host, each identified by a unique board ID.
type HCASpecs struct {
	HcaSpec map[string]*HCASpec `json:"hca" yaml:"hca"`
}

type HCASpec struct {
	Hardware collector.IBHardWareInfo `json:"hardware" yaml:"hardware"`
	Perf     HCAPerf                  `json:"perf" yaml:"perf"`
}

type HCAPerf struct {
	OneWayBW   float32 `json:"one_way_bw" yaml:"one_way_bw"`         // Gbps
	AvgLatency float32 `json:"avg_latency_us" yaml:"avg_latency_us"` // ns
}

// LoadSpec loads the HCA specifications from the provided file or from default locations.
func LoadSpec(file string) (*HCASpecs, error) {
	s := &HCASpecs{}
	// 1. Load spec from provided file
	if file != "" {
		err := s.tryLoadFromFile(file)
		if err == nil && s.HcaSpec != nil {
			return FilterSpecsForLocalHost(s)
		} else {
			logrus.WithField("component", "HCA").Warnf("%v", err)
		}
	}

	// 2. try to Load default spec from production env if no file specified
	err := s.tryLoadFromDefault()
	if err == nil {
		return FilterSpecsForLocalHost(s)
	} else {
		logrus.WithField("component", "HCA").Warnf("%v", err)
	}

	// 3. try to load default spec from default config directory based on caller path
	err = s.tryLoadFromDevConfig()
	if err == nil {
		return FilterSpecsForLocalHost(s)
	} else {
		logrus.WithField("component", "HCA").Warnf("%v", err)
	}

	return nil, fmt.Errorf("failed to load HCA spec from any source, please check the configuration")
}

func (s *HCASpecs) tryLoadFromFile(file string) error {
	if file == "" {
		return fmt.Errorf("file path is empty")
	}
	err := utils.LoadFromYaml(file, s)
	if err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}

	if s.HcaSpec == nil {
		return fmt.Errorf("YAML file %s loaded but contains no hca section", file)
	}
	logrus.WithField("component", "HCA").Infof("loaded default spec")
	return nil
}

func (s *HCASpecs) tryLoadFromDefault() error {
	specs := &HCASpecs{}
	err := common.LoadFromProductionDefaultSpec(specs)
	if err != nil || specs.HcaSpec == nil {
		return fmt.Errorf("%v", err)
	}
	if s.HcaSpec == nil {
		s.HcaSpec = make(map[string]*HCASpec)
	}

	for hcaName, spec := range specs.HcaSpec {
		if _, ok := s.HcaSpec[hcaName]; !ok {
			s.HcaSpec[hcaName] = spec
		}
	}
	logrus.WithField("component", "HCA").Infof("loaded default spec")
	return nil
}

func (s *HCASpecs) tryLoadFromDevConfig() error {
	defaultDevCfgDirPath, files, err := common.GetDevDefaultConfigFiles(consts.ComponentNameHCA)
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), consts.DefaultSpecSuffix) {
				specs := &HCASpecs{}
				filePath := filepath.Join(defaultDevCfgDirPath, file.Name())
				err := utils.LoadFromYaml(filePath, specs)
				if err != nil || specs.HcaSpec == nil {
					return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
				}
				if s.HcaSpec == nil {
					s.HcaSpec = make(map[string]*HCASpec)
				}
				for hcaName, hcaSpec := range specs.HcaSpec {
					if _, ok := s.HcaSpec[hcaName]; !ok {
						s.HcaSpec[hcaName] = hcaSpec
					}
				}
			}
		}
	}
	return err
}

// FilterSpecsForLocalHost retrieves the hca specification for the current host by checking the board IDs of the IB devices.
// It loads the specification from OSS if the board ID is not found in the current spec.
func FilterSpecsForLocalHost(allSpecs *HCASpecs) (*HCASpecs, error) {
	if allSpecs == nil || allSpecs.HcaSpec == nil {
		return nil, fmt.Errorf("HCA spec is not initialized")
	}
	// Get the board IDs of the IB devices in the host
	ibDevs, err := GetBoardIDs()
	if err != nil {
		return nil, err
	}

	result := &HCASpecs{HcaSpec: map[string]*HCASpec{}}
	// Check if the IBDevs in the spec have corresponding board IDs in host
	missing := []string{}

	for _, ibDevBoardId := range ibDevs {
		if spec, ok := allSpecs.HcaSpec[ibDevBoardId]; ok {
			// If the spec is found in the current spec, add it to the result
			result.HcaSpec[ibDevBoardId] = spec
		} else {
			// If the spec is not found in the current spec, try to load it from OSS
			logrus.WithField("component", "HCA").Warnf("spec for board ID %s not found in current spec, trying to load from OSS", ibDevBoardId)
			tmpSpecs := &HCASpecs{}
			url := fmt.Sprintf("%s/%s/%s.yaml", consts.DefaultOssCfgPath, consts.ComponentNameHCA, ibDevBoardId)
			logrus.WithField("component", "HCA").Infof("Loading spec from OSS for board ID %s: %s", ibDevBoardId, url)
			// Attempt to load spec from OSS
			err := common.LoadSpecFromOss(url, tmpSpecs)
			if err == nil && tmpSpecs.HcaSpec != nil {
				// If the spec is found in OSS, add it to the main spec
				if spec, ok := tmpSpecs.HcaSpec[ibDevBoardId]; ok {
					result.HcaSpec[ibDevBoardId] = spec
				} else {
					logrus.WithField("component", "HCA").Warnf("spec for board ID %s not found in OSS, skipping", ibDevBoardId)
					missing = append(missing, ibDevBoardId)
					continue
				}
			} else {
				logrus.WithField("component", "HCA").Errorf("failed to load spec from OSS for board ID %s: %v", ibDevBoardId, err)
				missing = append(missing, ibDevBoardId)
			}
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("spec for the following board IDs not found in any source: %v", common.ExtractAndDeduplicate(strings.Join(missing, ",")))
	}

	return result, nil
}

func GetBoardIDs() ([]string, error) {
	baseDir := "/sys/class/infiniband"
	devices, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", baseDir, err)
	}

	boardIDSet := make(map[string]struct{})
	for _, dev := range devices {
		boardIDPath := filepath.Join(baseDir, dev.Name(), "board_id")
		content, err := os.ReadFile(boardIDPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read board_id for device %s: %v\n", dev.Name(), err)
			continue
		}
		boardID := strings.TrimSpace(string(content))
		if boardID == "" {
			continue
		}
		boardIDSet[boardID] = struct{}{}
	}

	var boardIDs []string
	for id := range boardIDSet {
		boardIDs = append(boardIDs, id)
	}
	return boardIDs, nil
}
