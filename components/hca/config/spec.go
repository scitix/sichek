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
	OneWayBW   float64 `json:"one_way_bw" yaml:"one_way_bw"`         // Gbps
	AvgLatency float64 `json:"avg_latency_us" yaml:"avg_latency_us"` // ns
}

// EnsureSpec ensures that `file` contains spec entries for all IB Board IDs
// present on the local host.
//
// It reads sysfs to discover local Board IDs (no hardware drivers needed).
// For any Board ID not yet in `file`, it downloads the per-board spec from
// SICHEK_SPEC_URL and merges it into `file` (with backup and tracing).
//
// Call this after spec.EnsureSpecFile so that `file` is the cluster-level
// multi-board map before this function adds local-host entries.
func EnsureSpec(file string) (string, error) {
	const comp = "hca/spec"

	_, boardIDs, err := GetIBPFBoardIDs()
	if err != nil {
		return file, fmt.Errorf("EnsureSpec: cannot detect board IDs: %w", err)
	}

	// Find which board IDs are missing from the cluster-level file
	var s HCASpecs
	_ = common.LoadSpec(file, &s) // may be empty on first run

	var missing []string
	for _, bid := range boardIDs {
		if s.HcaSpec == nil || s.HcaSpec[bid] == nil {
			missing = append(missing, bid)
		}
	}

	if len(missing) == 0 {
		logrus.WithField("component", comp).Infof("all local board IDs %v already present in %s, skipping download", boardIDs, file)
		return file, nil
	}
	logrus.WithField("component", comp).Infof("board IDs %v are missing from %s, will try downloading", missing, file)

	ossBase := os.Getenv("SICHEK_SPEC_URL")
	if ossBase == "" {
		return file, fmt.Errorf("EnsureSpec: board IDs %v not in spec and SICHEK_SPEC_URL not set", missing)
	}

	for _, bid := range missing {
		perBoardURL := fmt.Sprintf("%s/%s/%s.yaml",
			strings.TrimRight(ossBase, "/"), consts.ComponentNameHCA, bid)
		tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s.yaml", bid))

		logrus.WithField("component", comp).Infof("downloading board ID %s spec from %s", bid, perBoardURL)
		if err := common.DownloadSpecFile(perBoardURL, tmpFile, comp); err != nil {
			logrus.WithField("component", comp).Warnf("download failed for %s: %v", bid, err)
			continue
		}

		var perBoard HCASpecs
		if err := common.LoadSpec(tmpFile, &perBoard); err != nil {
			logrus.WithField("component", comp).Warnf("parse failed for %s: %v", bid, err)
			continue
		}

		if err := common.MergeAndWriteSpec(
			file,
			"hca_specs",
			perBoard.HcaSpec,
			func(c *HCASpecs) map[string]*HCASpec { return c.HcaSpec },
			func(c *HCASpecs, m map[string]*HCASpec) { c.HcaSpec = m },
		); err != nil {
			logrus.WithField("component", comp).Warnf("merge failed for %s: %v", bid, err)
		}
	}
	return file, nil
}

// LoadSpec loads the HCA specifications from multiple sources.
// It automatically calls EnsureSpec to guarantee that all local Board IDs
// have an entry in `file` (potentially downloading from OSS if missing).
func LoadSpec(file string) (*HCASpecs, error) {
	// 1. Ensure all local board IDs are present in the file (OSS fallback)
	if file != "" {
		if _, err := EnsureSpec(file); err != nil {
			logrus.WithField("component", "hca/spec").Warnf("EnsureSpec failed: %v", err)
		}
	}

	s := &HCASpecs{}
	if s.HcaSpec == nil {
		s.HcaSpec = make(map[string]*HCASpec)
	}

	// 2. Load spec from provided file (highest priority)
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

	// 4. Filter specs for local host and load missing specs from remote SICHEK_SPEC_URL
	// This will check all board IDs on the host and ensure each has a spec
	result, err := FilterSpecsForLocalHost(file, s)
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
	if err := common.LoadSpec(file, tempSpecs); err != nil {
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

// FilterSpecsForLocalHost filters `allSpecs` to include only the board IDs
// present on the local host. If `file` is non-empty, overwrites it with the
// filtered subset (the applied baseline) using common.WriteSpec (.bak backup + tracing).
// This is a pure lookup; no network calls. If IDs are missing, call EnsureSpec first.
func FilterSpecsForLocalHost(file string, allSpecs *HCASpecs) (*HCASpecs, error) {
	if allSpecs == nil || allSpecs.HcaSpec == nil {
		return nil, fmt.Errorf("HCA spec is not initialized")
	}
	_, ibDevs, err := GetIBPFBoardIDs()
	if err != nil {
		return nil, err
	}

	result := &HCASpecs{HcaSpec: map[string]*HCASpec{}}
	var missing []string

	for _, boardID := range ibDevs {
		if spec, ok := allSpecs.HcaSpec[boardID]; ok {
			result.HcaSpec[boardID] = spec
		} else {
			logrus.WithField("component", "hca").Warnf(
				"spec for board ID %s not found; call EnsureSpec first", boardID)
			missing = append(missing, boardID)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("spec not found for board IDs: %v; call EnsureSpec first or set SICHEK_SPEC_URL",
			common.ExtractAndDeduplicate(strings.Join(missing, ",")))
	}

	// Persist the applied baseline (all local board IDs' specs)
	if file != "" {
		if err := common.WriteSpec(file, "hca_specs", "hca/spec", result); err != nil {
			logrus.WithField("component", "hca").Warnf("failed to write applied baseline: %v", err)
		}
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
		// if strings.Contains(devName, "bond") {
		// 	continue // Skip bonding devices
		// }
		if strings.Contains(devName, "mezz") {
			continue // Skip mezzanine card
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
