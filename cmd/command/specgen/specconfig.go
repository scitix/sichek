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
package specgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	hcaSpec "github.com/scitix/sichek/components/hca/config"
	infinibandSpec "github.com/scitix/sichek/components/infiniband/config"
	nvidiaSpec "github.com/scitix/sichek/components/nvidia/config"
	pcieSpec "github.com/scitix/sichek/components/pcie/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type SpecConfig struct {
	Nvidia     map[string]nvidiaSpec.NvidiaSpec         `yaml:"nvidia"`
	Infiniband map[string]infinibandSpec.InfinibandSpec `yaml:"infiniband"`
	HCA        map[string]hcaSpec.HCASpec               `yaml:"hca"`
	PCIeTopo   map[string]pcieSpec.PcieTopoSpec         `yaml:"pcie_topo"`
	// Internal fields
	sourceFile string
	isFiltered bool
}

// NewSpecConfigCmd creates the spec config command
func NewSpecConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage spec configurations",
		Long:  "Create, view, and upload spec configurations for different cluster environments",
	}

	cmd.AddCommand(NewSpecConfigCreateCmd())
	cmd.AddCommand(NewSpecConfigViewCmd())
	cmd.AddCommand(NewSpecConfigUploadCmd())

	return cmd
}

// NewSpecConfigCreateCmd creates the spec config create command
func NewSpecConfigCreateCmd() *cobra.Command {
	var outputFile string
	var specName string
	var noUpload bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new spec configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use the output file path as provided, or default to production path
			var outputFullPath string
			if filepath.IsAbs(outputFile) || strings.Contains(outputFile, "/") {
				// If it's an absolute path or contains path separators, use as-is
				outputFullPath = outputFile
			} else {
				// If it's just a filename, save to production path
				outputFullPath = filepath.Join(consts.DefaultProductionCfgPath, outputFile)
			}

			// Check if spec file already exists
			var spec *SpecConfig
			var isExisting bool
			if _, err := os.Stat(outputFullPath); err == nil {
				// File exists, load it
				spec, err = LoadSpecFromFile(outputFullPath)
				if err != nil {
					return fmt.Errorf("failed to load existing spec from %s: %v", outputFullPath, err)
				}
				// Ensure all maps are initialized
				if spec.Nvidia == nil {
					spec.Nvidia = make(map[string]nvidiaSpec.NvidiaSpec)
				}
				if spec.Infiniband == nil {
					spec.Infiniband = make(map[string]infinibandSpec.InfinibandSpec)
				}
				if spec.HCA == nil {
					spec.HCA = make(map[string]hcaSpec.HCASpec)
				}
				if spec.PCIeTopo == nil {
					spec.PCIeTopo = make(map[string]pcieSpec.PcieTopoSpec)
				}
				isExisting = true
				fmt.Printf("📁 Found existing spec file: %s\n", outputFullPath)
			} else {
				// File doesn't exist, create new spec
				spec = CreateDefaultSpec()
				isExisting = false
				fmt.Println("✅ Created new spec template")
			}

			// Interactive component selection
			fmt.Println("\n🔧 Spec Configuration Setup")
			fmt.Println("=" + strings.Repeat("=", 40))
			fmt.Println("Select which components to configure:")
			fmt.Println()

			// Component selection
			components := []string{"nvidia", "infiniband", "hca", "pcie_topo"}
			selectedComponents := make([]string, 0)

			for _, component := range components {
				enabled := promptBool(fmt.Sprintf("Configure %s?", component), true)
				if enabled {
					selectedComponents = append(selectedComponents, component)
				}
			}

			if len(selectedComponents) == 0 {
				fmt.Println("⚠️  No components selected. Skipping configuration.")
			} else {
				fmt.Printf("\n✅ Selected components: %s\n", strings.Join(selectedComponents, ", "))
				fmt.Println()

				// Configure each selected component
				for _, component := range selectedComponents {
					fmt.Printf("🔧 Configuring %s...\n", component)
					switch component {
					case "nvidia":
						nvidiaSpecs := FillNvidiaSpec()
						for id, nvidiaSpec := range nvidiaSpecs {
							spec.Nvidia[id] = nvidiaSpec
						}
					case "infiniband":
						infinibandSpecs := FillInfinibandSpec()
						for id, infinibandSpec := range infinibandSpecs {
							spec.Infiniband[id] = infinibandSpec
						}
					case "hca":
						hcaSpecs := FillHcaSpec()
						for id, hcaSpec := range hcaSpecs {
							spec.HCA[id] = hcaSpec
						}
					case "pcie_topo":
						pcieTopoSpecs := FillPcieTopoSpec()
						for id, pcieTopoSpec := range pcieTopoSpecs {
							spec.PCIeTopo[id] = pcieTopoSpec
						}
					}
					fmt.Println()
				}
			}

			// Save to file
			if err := SaveSpecToFile(spec, outputFullPath); err != nil {
				return fmt.Errorf("failed to save spec to %s: %v", outputFullPath, err)
			}

			if isExisting {
				fmt.Printf("✅ Updated spec configuration saved to %s\n", outputFullPath)
			} else {
				fmt.Printf("✅ Spec configuration saved to %s\n", outputFullPath)
			}

			// Auto upload to SICHEK_SPEC_URL (default behavior, unless --no-upload is specified)
			if !noUpload {
				if specName == "" {
					// Use filename as spec name
					specName = filepath.Base(outputFile)
				}

				fmt.Println("\nUploading to SICHEK_SPEC_URL...")
				specURL, err := UploadSpec(spec, specName)
				if err != nil {
					fmt.Printf("⚠️  Failed to upload to SICHEK_SPEC_URL: %v\n", err)
					fmt.Println("   You can upload manually later with: sichek spec upload --file", outputFile)
				} else {
					fmt.Printf("✅ Spec uploaded to SICHEK_SPEC_URL: %s\n", specURL)
				}
			}

			fmt.Println("\n📝 Next steps:")
			fmt.Println("1. Use 'sichek spec view' to preview the configuration")
			if noUpload {
				fmt.Println("2. Use 'sichek spec upload' to upload to SICHEK_SPEC_URL")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "spec-filename", "f", "", "Output file path (required)")
	cmd.Flags().BoolVar(&noUpload, "no-upload", false, "Skip automatic upload to SICHEK_SPEC_URL")
	cmd.MarkFlagRequired("spec-filename")

	return cmd
}

// NewSpecConfigViewCmd creates the spec config view command
func NewSpecConfigViewCmd() *cobra.Command {
	var format string
	var showSections []string

	cmd := &cobra.Command{
		Use:   "view [file|spec-name]",
		Short: "View spec configuration",
		Long:  "View spec configuration from file, SICHEK_SPEC_URL spec name, or current system",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var spec *SpecConfig
			var err error

			if len(args) > 0 {
				// Load from specified file, URL, or spec name
				input := args[0]
				if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
					// Direct URL
					spec, err = LoadSpecFromURL(input)
					if err != nil {
						return fmt.Errorf("failed to load spec from url %s: %v", input, err)
					}
					fmt.Printf("✅ Loaded spec from url: %s\n", input)
				} else if strings.Contains(input, "/") || strings.Contains(input, ".") {
					// File path (contains path separator or file extension)
					spec, err = LoadSpecFromFile(input)
					if err != nil {
						return fmt.Errorf("failed to load spec from file %s: %v", input, err)
					}
					fmt.Printf("✅ Loaded spec from file: %s\n", input)
				}
			} else {
				// Try to load from default locations
				defaultPaths := []string{
					filepath.Join(consts.DefaultProductionCfgPath, consts.DefaultSpecCfgName),
				}

				var loaded bool
				for _, path := range defaultPaths {
					if _, err := os.Stat(path); err == nil {
						spec, err = LoadSpecFromFile(path)
						if err == nil {
							fmt.Printf("✅ Loaded spec from: %s\n", path)
							loaded = true
							break
						}
					}
				}

				if !loaded {
					// Try to load from remote SICHEK_SPEC_URL
					specURL := httpclient.GetSichekSpecURL()
					if specURL == "" {
						return fmt.Errorf("SICHEK_SPEC_URL environment variable is not set, cannot load spec from SICHEK_SPEC_URL")
					}
					specUrl := specURL + "/" + consts.DefaultSpecCfgName
					fmt.Printf("No local spec file found, checking spec from url: %s\n...", specUrl)
					spec, err = LoadSpecFromURL(specUrl)
					if err != nil {
						return fmt.Errorf("failed to load spec from url %s: %v", specUrl, err)
					}
				}
			}

			// Filter sections if specified
			if len(showSections) > 0 {
				spec = specSections(spec, showSections)
			}

			// Print the spec
			if err := PrintSpec(spec, format); err != nil {
				return fmt.Errorf("failed to print spec: %v", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "structured", "Output format: yaml, json, structured, raw-yaml")
	cmd.Flags().StringSliceVarP(&showSections, "sections", "s", []string{}, "Show only specific sections: nvidia, infiniband, hca, pcie_topo")

	return cmd
}

// Helper function to spec sections
func specSections(spec *SpecConfig, sections []string) *SpecConfig {
	specSections := &SpecConfig{
		sourceFile: spec.sourceFile, // Preserve source file path
		isFiltered: true,            // Mark as filtered
	}

	for _, section := range sections {
		switch section {
		case "nvidia":
			specSections.Nvidia = spec.Nvidia
		case "infiniband":
			specSections.Infiniband = spec.Infiniband
		case "hca":
			specSections.HCA = spec.HCA
		case "pcie_topo":
			specSections.PCIeTopo = spec.PCIeTopo
		}
	}

	return specSections
}

// LoadSpecFromFile loads spec configuration from a YAML file
func LoadSpecFromFile(filePath string) (*SpecConfig, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	spec := &SpecConfig{}
	err := utils.LoadFromYaml(filePath, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to load YAML from %s: %v", filePath, err)
	}

	// Ensure all map fields are initialized (not nil) to prevent them from being omitted during marshaling
	if spec.Nvidia == nil {
		spec.Nvidia = make(map[string]nvidiaSpec.NvidiaSpec)
	}
	if spec.Infiniband == nil {
		spec.Infiniband = make(map[string]infinibandSpec.InfinibandSpec)
	}
	if spec.HCA == nil {
		spec.HCA = make(map[string]hcaSpec.HCASpec)
	}
	if spec.PCIeTopo == nil {
		spec.PCIeTopo = make(map[string]pcieSpec.PcieTopoSpec)
	}

	// Save source file path for raw YAML printing
	spec.sourceFile = filePath

	return spec, nil
}

// SaveSpecToFile saves spec configuration to a YAML file
func SaveSpecToFile(spec *SpecConfig, filePath string) error {
	if spec == nil {
		return fmt.Errorf("spec configuration is nil")
	}

	if filePath == "" {
		return fmt.Errorf("output file path is empty")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Marshal to YAML with 2-space indentation
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	err := encoder.Encode(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec to YAML: %v", err)
	}
	encoder.Close()
	data := buf.Bytes()

	// Write to file
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write spec to file %s: %v", filePath, err)
	}

	return nil
}

// CreateDefaultSpec creates a default spec configuration
func CreateDefaultSpec() *SpecConfig {
	return &SpecConfig{
		Nvidia:     make(map[string]nvidiaSpec.NvidiaSpec),
		Infiniband: make(map[string]infinibandSpec.InfinibandSpec),
		HCA:        make(map[string]hcaSpec.HCASpec),
		PCIeTopo:   make(map[string]pcieSpec.PcieTopoSpec),
	}
}

// LoadSpecFromURL loads spec configuration from URL
func LoadSpecFromURL(url string) (*SpecConfig, error) {
	if url == "" {
		return nil, fmt.Errorf("Spec URL is empty")
	}

	var spec SpecConfig
	if err := httpclient.LoadSpecFromURL(url, &spec); err != nil {
		return nil, fmt.Errorf("failed to load spec from URL %s: %v", url, err)
	}

	return &spec, nil
}

// PrintSpec prints spec configuration in the specified format
func PrintSpec(spec *SpecConfig, format string) error {
	if spec == nil {
		return fmt.Errorf("spec configuration is nil")
	}

	switch format {
	case "yaml":
		data, err := yaml.Marshal(spec)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %v", err)
		}
		fmt.Print(string(data))

	case "raw-yaml":
		// Print raw YAML content from the original file
		return printRawYAML(spec)

	case "json":
		data, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal to JSON: %v", err)
		}
		fmt.Print(string(data))

	case "structured":
		printStructuredSpec(spec)

	default:
		return fmt.Errorf("unsupported format: %s. Supported formats: yaml, json, structured, raw-yaml", format)
	}

	return nil
}

// printStructuredSpec prints spec in a structured, human-readable format
func printStructuredSpec(spec *SpecConfig) {
	fmt.Println("📋 Spec Configuration")
	fmt.Println("=" + strings.Repeat("=", 50))

	// NVIDIA section
	if len(spec.Nvidia) > 0 {
		fmt.Println("\n🧩 NVIDIA Configuration:")
		for id, nvidia := range spec.Nvidia {
			fmt.Printf("  GPU ID: %s\n", id)
			fmt.Printf("    Name: %s\n", nvidia.Name)
			fmt.Printf("    GPU Count: %d\n", nvidia.GpuNums)
			fmt.Printf("    Memory: %d GB\n", nvidia.GpuMemory)
			fmt.Printf("    Driver: %s\n", nvidia.Software.DriverVersion)
			fmt.Printf("    CUDA: %s\n", nvidia.Software.CUDAVersion)
		}
	}

	// Infiniband section
	if len(spec.Infiniband) > 0 {
		fmt.Println("\n🌐 Infiniband Configuration:")
		for id, ib := range spec.Infiniband {
			fmt.Printf("  IB ID: %s\n", id)
			if ib.IBSoftWareInfo != nil {
				fmt.Printf("    OFED Version: %s\n", ib.IBSoftWareInfo.OFEDVer)
			}
			fmt.Printf("    PCIe ACS: %s\n", ib.PCIeACS)
			fmt.Printf("    IB Devices: %d\n", len(ib.IBPFDevs))
		}
	}

	// HCA section
	if len(spec.HCA) > 0 {
		fmt.Println("\n💽 HCA Configuration:")
		for id, hca := range spec.HCA {
			fmt.Printf("  HCA ID: %s\n", id)
			fmt.Printf("    Board ID: %s\n", hca.Hardware.BoardID)
			fmt.Printf("    HCA Type: %s\n", hca.Hardware.HCAType)
			fmt.Printf("    Firmware: %s\n", hca.Hardware.FWVer)
			fmt.Printf("    Port Speed: %s\n", hca.Hardware.PortSpeed)
		}
	}

	// PCIe Topology section
	if len(spec.PCIeTopo) > 0 {
		fmt.Println("\n🧮 PCIe Topology Configuration:")
		for id, pcie := range spec.PCIeTopo {
			fmt.Printf("  GPU PCI ID: %s\n", id)
			fmt.Printf("    NUMA Nodes: %d\n", len(pcie.NumaConfig))
			fmt.Printf("    PCI Switches: %d\n", len(pcie.PciSwitchesConfig))
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 52))
}

// printRawYAML prints the original YAML content from the source file
func printRawYAML(spec *SpecConfig) error {
	if spec.sourceFile == "" {
		return fmt.Errorf("no source file path available for raw YAML printing")
	}

	// If sections are filtered, we need to parse and filter the YAML content
	// For now, we'll fall back to structured format for section filtering
	if spec.isFiltered {
		fmt.Println("⚠️  Note: Section filtering with raw-yaml format is not fully supported.")
		fmt.Println("   Falling back to structured format for filtered sections.")
		printStructuredSpec(spec)
		return nil
	}

	// Read the original file content
	content, err := os.ReadFile(spec.sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %v", spec.sourceFile, err)
	}

	// Print the raw content
	fmt.Print(string(content))
	return nil
}

// NewSpecConfigUploadCmd creates the spec config upload command
func NewSpecConfigUploadCmd() *cobra.Command {
	var specFile string

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload spec configuration to remote SICHEK_SPEC_URL",
		Long:  "Upload a spec configuration file to the remote URL (specified by SICHEK_SPEC_URL) for sharing and distribution",
		RunE: func(cmd *cobra.Command, args []string) error {
			if specFile == "" {
				return fmt.Errorf("spec file path is required. Use --file flag to specify the file")
			}

			// Use filename without extension as spec name
			specName := filepath.Base(specFile)

			// Load the spec file
			spec, err := LoadSpecFromFile(specFile)
			if err != nil {
				return fmt.Errorf("failed to load spec from %s: %v", specFile, err)
			}

			// Upload spec
			specURL, err := UploadSpec(spec, specName)
			if err != nil {
				return fmt.Errorf("failed to upload spec: %v", err)
			}

			fmt.Printf("✅ Spec configuration uploaded successfully\n")
			fmt.Printf("   Spec Name: %s\n", specName)
			fmt.Printf("   Spec URL: %s\n", specURL)
			fmt.Println("\n  Next steps:")
			fmt.Println("1. Use 'sichek spec list' to see all available specs")
			fmt.Println("2. Use 'sichek spec view <url>' to view the uploaded spec")

			return nil
		},
	}

	cmd.Flags().StringVarP(&specFile, "file", "f", "", "Spec file path to upload (required)")
	cmd.MarkFlagRequired("file")
	return cmd
}

// UploadSpec uploads a spec configuration to the remote URL specified by SICHEK_SPEC_URL
func UploadSpec(spec *SpecConfig, specName string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("spec configuration is nil")
	}

	if specName == "" {
		return "", fmt.Errorf("spec name is required")
	}

	// Marshal spec to YAML
	data, err := yaml.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal spec to YAML: %v", err)
	}

	// Use http client to upload
	specURL := httpclient.GetSichekSpecURL()
	if specURL == "" {
		return "", fmt.Errorf("SICHEK_SPEC_URL environment variable is not set, cannot upload spec to SICHEK_SPEC_URL")
	}
	specUrl := specURL + "/" + specName
	err = httpclient.Upload(specUrl, data)
	if err != nil {
		return "", fmt.Errorf("failed to upload spec to remote url %s: %v", specUrl, err)
	}

	return specUrl, nil
}
