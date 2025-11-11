package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/oss"
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
	OneWayBW   float64 `json:"one_way_bw" yaml:"one_way_bw"`         // Gbps
	AvgLatency float64 `json:"avg_latency_us" yaml:"avg_latency_us"` // ns
}

// LoadSpec loads the HCA specifications from the provided file and merges with default locations.
// The provided file has higher priority - if the same board ID exists in both, the provided file's spec will be used.
// After merging, it filters the specs for the local host and loads missing specs from OSS if needed.
func LoadSpec(file string) (*HCASpecs, error) {
	s := &HCASpecs{}
	if s.HcaSpec == nil {
		s.HcaSpec = make(map[string]*HCASpec)
	}

	// 1. Load spec from provided file (highest priority)
	if file != "" {
		err := s.tryLoadFromFile(file)
		if err != nil {
			logrus.WithField("component", "hca").Warnf("failed to load spec from provided file %s: %v", file, err)
		} else if len(s.HcaSpec) > 0 {
			logrus.WithField("component", "hca").Infof("loaded HCA spec from provided file: %s", file)
		}
	}

	// 2. Try to load default spec from production env and merge
	// e.g., /var/sichek/config/default_spec.yaml
	// The provided file's specs take precedence (already loaded, won't be overwritten)
	err := s.tryLoadFromDefault()
	if err != nil {
		logrus.WithField("component", "hca").Warnf("failed to load default production spec: %v", err)
	} else if len(s.HcaSpec) > 0 {
		logrus.WithField("component", "hca").Infof("merged default production HCA spec")
	}

	// 3. Try to load default spec from default config directory and merge
	// for production env, it checks the default config path (e.g., /var/sichek/config/xx-component).
	// for development env, it checks the default config path based on runtime.Caller  (e.g., /repo/component/xx-component/config).
	// The provided file's specs take precedence (already loaded, won't be overwritten)
	err = s.tryLoadFromDevConfig()
	if err != nil {
		logrus.WithField("component", "hca").Warnf("failed to load from default dev directory: %v", err)
	} else if len(s.HcaSpec) > 0 {
		logrus.WithField("component", "hca").Infof("merged default dev HCA spec")
	}

	// Check if we have any HCA specs loaded
	if len(s.HcaSpec) == 0 {
		return nil, fmt.Errorf("failed to load HCA spec from any source, please check the configuration")
	}

	// 4. Filter specs for local host and load missing specs from OSS
	// This will check all board IDs on the host and ensure each has a spec
	result, err := FilterSpecsForLocalHost(s)
	if err != nil {
		return nil, fmt.Errorf("failed to filter HCA specs for local host: %w", err)
	}

	logrus.WithField("component", "hca").Infof("successfully loaded and merged HCA specs, total board IDs: %d", len(result.HcaSpec))
	return result, nil
}

func (s *HCASpecs) tryLoadFromFile(file string) error {
	if file == "" {
		return fmt.Errorf("file path is empty")
	}
	tempSpecs := &HCASpecs{}
	err := utils.LoadFromYaml(file, tempSpecs)
	if err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}
	if tempSpecs.HcaSpec == nil {
		return fmt.Errorf("YAML file %s loaded but contains no hca section", file)
	}
	// Merge into the main spec (provided file has highest priority, so overwrite existing)
	if s.HcaSpec == nil {
		s.HcaSpec = make(map[string]*HCASpec)
	}
	for hcaName, spec := range tempSpecs.HcaSpec {
		s.HcaSpec[hcaName] = spec // Overwrite if exists (provided file has priority)
	}
	return nil
}

func (s *HCASpecs) tryLoadFromDefault() error {
	specs := &HCASpecs{}
	err := common.LoadSpecFromProductionPath(specs)
	if err != nil {
		return err
	}
	if specs.HcaSpec == nil {
		return fmt.Errorf("default production top spec loaded but contains no hca section")
	}
	// Merge the loaded specs with the existing ones
	if s.HcaSpec == nil {
		s.HcaSpec = make(map[string]*HCASpec)
	}

	for hcaName, spec := range specs.HcaSpec {
		if _, ok := s.HcaSpec[hcaName]; !ok {
			s.HcaSpec[hcaName] = spec
		}
	}
	logrus.WithField("component", "hca").Infof("loaded default production top spec")
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
					// If the file is not found or does not contain HCA specs, log the error
					// and continue to the next file.
					logrus.WithField("component", "hca").Warnf("failed to load HCA spec from YAML file %s: %v", filePath, err)
					continue
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
	_, ibDevs, err := GetIBPFBoardIDs()
	if err != nil {
		return nil, err
	}

	result := &HCASpecs{HcaSpec: map[string]*HCASpec{}}
	// Check if the IBPFDevs in the spec have corresponding board IDs in host
	missing := []string{}

	for _, ibDevBoardId := range ibDevs {
		if spec, ok := allSpecs.HcaSpec[ibDevBoardId]; ok {
			// If the spec is found in the current spec, add it to the result
			result.HcaSpec[ibDevBoardId] = spec
		} else {
			// If the spec is not found in the current spec, try to load it from OSS
			logrus.WithField("component", "hca").Warnf("spec for board ID %s not found in current spec, trying to load from OSS", ibDevBoardId)
			tmpSpecs := &HCASpecs{}
			url := fmt.Sprintf("%s/%s/%s.yaml", consts.DefaultOssCfgPath, consts.ComponentNameHCA, ibDevBoardId)
			logrus.WithField("component", "hca").Infof("Loading spec from OSS for board ID %s: %s", ibDevBoardId, url)
			// Attempt to load spec from OSS
			err := oss.LoadSpecFromURL(url, tmpSpecs)
			if err == nil && tmpSpecs.HcaSpec != nil {
				// If the spec is found in OSS, add it to the main spec
				if spec, ok := tmpSpecs.HcaSpec[ibDevBoardId]; ok {
					result.HcaSpec[ibDevBoardId] = spec
				} else {
					logrus.WithField("component", "hca").Warnf("spec for board ID %s not found in OSS, skipping", ibDevBoardId)
					missing = append(missing, ibDevBoardId)
					continue
				}
			} else {
				logrus.WithField("component", "hca").Errorf("failed to load spec from OSS for board ID %s: %v", ibDevBoardId, err)
				missing = append(missing, ibDevBoardId)
			}
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("spec for the following board IDs not found in any source: %v", common.ExtractAndDeduplicate(strings.Join(missing, ",")))
	}

	return result, nil
}

func GetIBPFBoardIDs() (map[string]string, []string, error) {
	baseDir := "/sys/class/infiniband"
	devices, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %v", baseDir, err)
	}

	boardIDSet := make(map[string]struct{})
	devBoardIDMap := make(map[string]string)
	for _, dev := range devices {
		devName := dev.Name()
		vfPath := filepath.Join(baseDir, devName, "device", "physfn")
		if _, err := os.Stat(vfPath); err == nil {
			continue // Skip virtual functions
		}
		if strings.Contains(devName, "bond") {
			continue // Skip bonding devices
		}
		boardIDPath := filepath.Join(baseDir, devName, "board_id")
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
		devBoardIDMap[devName] = boardID
	}

	var boardIDs []string
	for id := range boardIDSet {
		boardIDs = append(boardIDs, id)
	}
	return devBoardIDMap, boardIDs, nil
}
