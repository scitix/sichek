package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/scitix/sichek/config/cpu"
	"github.com/scitix/sichek/config/dmesg"
	"github.com/scitix/sichek/config/ethernet"
	"github.com/scitix/sichek/config/gpfs"
	"github.com/scitix/sichek/config/hca"
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/config/memory"
	"github.com/scitix/sichek/config/nccl"
	"github.com/scitix/sichek/config/hang"
	"github.com/scitix/sichek/config/nvidia"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type ComponentConfig struct {
	componentBasicConfig *BasicComponentConfigs
	componentSpecConfig  *SpecComponentConfigs
}
type BasicComponentConfigs struct {
	cpuBasicConfig        *cpu.CPUConfig               `json:"cpu" yaml:"cpu"`
	dmesgBasicConfig      *dmesg.DmesgConfig           `json:"dmesg" yaml:"dmesg"`
	gpfsBasicConfig       *gpfs.GpfsConfig             `json:"gpfs" yaml:"gpfs"`
	ncclBasicConfig       *nccl.NCCLConfig             `json:"nccl" yaml:"nccl"`
	memoryBasicConfig     *memory.MemoryConfig         `json:"memory" yaml:"memory"`
	hangBasicConfig       *hang.HangConfig             `json:"hang" yaml:"hang"`
	ethernetBasicConfig   *ethernet.EthernetConfig     `json:"ethernet" yaml:"ethernet"`
	nvidiaBasicConfig     *nvidia.NvidiaConfig         `json:"nvidia" yaml:"nvidia"`
	infinibandBasicConfig *infiniband.InfinibandConfig `json:"infiniband" yaml:"infiniband"`
}

type SpecComponentConfigs struct {
	nvidiaSpecConfig     *nvidia.NvidiaSpec         `json:"nvidia" yaml:"nvidia"`
	infinibandSpecConfig *infiniband.InfinibandSpec `json:"infiniband" yaml:"infiniband"`
	hcaSpecConfig        *hca.HCASpec               `json:"hca" yaml:"hca"`
	ethernetSpecConfig   *ethernet.EthernetSpec     `json:"ethernet" yaml:"ethernet"`
}

var (
	instance *ComponentConfig
	once     sync.Once
)

// 默认初始化函数
func LoadComponentConfig(basicConfigPath, specConfigPath string) (*ComponentConfig, error) {
	once.Do(func() {
		instance = &ComponentConfig{
			componentBasicConfig: &BasicComponentConfigs{},
			componentSpecConfig:  &SpecComponentConfigs{},
		}
		if err := utils.LoadFromYaml(basicConfigPath, instance.componentBasicConfig); err != nil {
			logrus.Warnf("[LoadComponentConfig] load basic config failed: %v", err)
			return
		}
		if err := utils.LoadFromYaml(specConfigPath, instance.componentSpecConfig); err != nil {
			logrus.Warnf("[LoadComponentConfig] load spec config failed: %v", err)
			return
		}
	})
	return instance, nil
}

func (c *ComponentConfig) GetConfigByComponentName(componentName string) (interface{}, interface{}) {
	switch componentName {
	case consts.ComponentNameCPU:
		if c.componentBasicConfig.cpuBasicConfig == nil {
			cpuConfig := &cpu.CPUConfig{}
			err := DefaultComponentConfig(componentName, cpuConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.cpuBasicConfig = cpuConfig
		}
		return c.componentBasicConfig.cpuBasicConfig, nil
	case consts.ComponentNameDmesg:
		if c.componentBasicConfig.dmesgBasicConfig == nil {
			dmesgConfig := &dmesg.DmesgConfig{}
			err := DefaultComponentConfig(componentName, dmesgConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.dmesgBasicConfig = dmesgConfig
		}
		return c.componentBasicConfig.dmesgBasicConfig, nil
	case consts.ComponentNameGpfs:
		if c.componentBasicConfig.gpfsBasicConfig == nil {
			gpfsConfig := &gpfs.GpfsConfig{}
			err := DefaultComponentConfig(componentName, gpfsConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.gpfsBasicConfig = gpfsConfig
		}
		return c.componentBasicConfig.gpfsBasicConfig, nil
	case consts.ComponentNameNCCL:
		if c.componentBasicConfig.ncclBasicConfig == nil {
			ncclConfig := &nccl.NCCLConfig{}
			err := DefaultComponentConfig(componentName, ncclConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.ncclBasicConfig = ncclConfig
		}
		return c.componentBasicConfig.ncclBasicConfig, nil
	case consts.ComponentNameMemory:
		if c.componentBasicConfig.memoryBasicConfig == nil {
			memoryConfig := &memory.MemoryConfig{}
			err := DefaultComponentConfig(componentName, memoryConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.memoryBasicConfig = memoryConfig
		}
		return c.componentBasicConfig.memoryBasicConfig, nil
	case consts.ComponentNameHang:
		if c.componentBasicConfig.hangBasicConfig == nil {
			hangConfig := &hang.HangConfig{}
			err := DefaultComponentConfig(componentName, hangConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.hangBasicConfig = hangConfig
		}
		return c.componentBasicConfig.hangBasicConfig, nil
	case consts.ComponentNameEthernet:
		if c.componentBasicConfig.ethernetBasicConfig == nil {
			ethernetConfig := &ethernet.EthernetConfig{}
			err := DefaultComponentConfig(componentName, ethernetConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.ethernetBasicConfig = ethernetConfig
		}
		if c.componentSpecConfig.ethernetSpecConfig == nil {
			ethernetSpecConfig := &ethernet.EthernetSpec{}
			err := DefaultComponentConfig(componentName, ethernetSpecConfig, consts.DefaultSpecCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentSpecConfig.ethernetSpecConfig = ethernetSpecConfig
		}
		return c.componentBasicConfig.ethernetBasicConfig, c.componentSpecConfig.ethernetSpecConfig
	case consts.ComponentNameNvidia:
		if c.componentBasicConfig.nvidiaBasicConfig == nil {
			nvidiaConfig := &nvidia.NvidiaConfig{}
			err := DefaultComponentConfig(componentName, nvidiaConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.nvidiaBasicConfig = nvidiaConfig
		}
		if c.componentSpecConfig.nvidiaSpecConfig == nil {
			nvidiaSpecConfig := &nvidia.NvidiaSpec{}
			err := DefaultComponentConfig(componentName, nvidiaSpecConfig, consts.DefaultSpecCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentSpecConfig.nvidiaSpecConfig = nvidiaSpecConfig
		}
		return c.componentBasicConfig.nvidiaBasicConfig, c.componentSpecConfig.nvidiaSpecConfig
	case consts.ComponentNameInfiniband:
		if c.componentBasicConfig.infinibandBasicConfig == nil {
			infinibandConfig := &infiniband.InfinibandConfig{}
			err := DefaultComponentConfig(componentName, infinibandConfig, consts.DefaultBasicCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentBasicConfig.infinibandBasicConfig = infinibandConfig
		}
		if c.componentSpecConfig.infinibandSpecConfig == nil {
			infinibandSpecConfig := &infiniband.InfinibandSpec{}
			err := DefaultComponentConfig(componentName, infinibandSpecConfig, consts.DefaultSpecCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentSpecConfig.infinibandSpecConfig = infinibandSpecConfig
		}
		return c.componentBasicConfig.infinibandBasicConfig, c.componentSpecConfig.infinibandSpecConfig
	case consts.ComponentNameHCA:
		if c.componentSpecConfig.hcaSpecConfig == nil {
			hcaSpecConfig := &hca.HCASpec{}
			err := DefaultComponentConfig(componentName, hcaSpecConfig, consts.DefaultSpecCfgName)
			if err != nil {
				return nil, nil
			}
			c.componentSpecConfig.hcaSpecConfig = hcaSpecConfig
		}
		return nil, c.componentSpecConfig.hcaSpecConfig
	default:
		return nil, fmt.Errorf("component %s not found", componentName)
	}
}

func DefaultComponentConfig(component string, config interface{}, filename string) error {
	defaultCfgPath := filepath.Join(consts.DefaultPodCfgPath, component, filename)
	_, err := os.Stat(defaultCfgPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// 获取当前文件的目录

		defaultCfgPath = filepath.Join(filepath.Dir(curFile), component, filename)
	}
	err = utils.LoadFromYaml(defaultCfgPath, config)
	return err
}
