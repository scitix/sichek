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
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewGpuCmd创建并返回用于代表GPU相关操作的子命令实例，配置命令的基本属性
func NewNvidiaCmd() *cobra.Command {

	NvidaCmd := &cobra.Command{
		Use:     "gpu",
		Aliases: []string{"g"},
		Short:   "Perform Nvidia - related operations",
		Long:    "Used to perform specific Nvidia - related operations, with specific functions to be expanded",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), CmdTimeout)
			verbos, err := cmd.Flags().GetBool("verbos")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the verbose: %v", err)
			}
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("components", "Nvidia").Info(fmt.Printf("Run NVIDIA HealthCheck Cmd context canceled"))
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("components", "Nvidia").Error(err)
			} else {
				logrus.WithField("components", "Nvidia").Info("load default cfg...")
			}

			ignored_checkers_str, err := cmd.Flags().GetString("ignored-checkers")
			if err != nil {
				logrus.WithField("components", "Nvidia").Error(err)
			} else {
				logrus.WithField("components", "Nvidia").Info("ignore checkers", ignored_checkers_str)
			}
			ignored_checkers := strings.Split(ignored_checkers_str, ",")

			component, err := nvidia.NewComponent(cfgFile, ignored_checkers)
			if err != nil {
				logrus.WithField("components", "Nvidia").Error("fail to Create Nvidia Components")
			}
			result, err := component.HealthCheck(ctx)
			if err != nil {
				logrus.WithField("component", component.Name()).Error(err)
				return
			}

			logrus.WithField("component", component.Name()).Infof("Analysis Result: %s\n", common.ToString(result))
			info, err := component.LastInfo(ctx)
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := PrintNvidiaInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[commonCfg.ComponentNameNvidia] = pass
			StatusMutex.Unlock()
		},
	}

	NvidaCmd.Flags().StringP("cfg", "c", "", "Path to the Nvidia Cfg")
	NvidaCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")
	NvidaCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return NvidaCmd
}

func PrintNvidiaInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	nvidiaInfo, ok := info.(*collector.NvidiaInfo)
	if !ok {
		logrus.WithField("component", "cpu").Errorf("invalid data type, expected NvidiaInfo")
		return false
	}
	checkerResults := result.Checkers
	var (
		driverPrint        string
		iommuPrint         string
		persistencePrint   string
		cudaVersionPrint   string
		acsPrint           string
		nvlinkPrint        string
		pcieLinkPrint      string
		peermemPrint       string
		pstatePrint        string
		gpuStatusPrint     string
		fabricmanagerPrint string
	)
	systemEvent := make(map[string]string)
	gpuStatus := make(map[string]string)
	clockEvents := make(map[string]string)
	eccEvents := make(map[string]string)
	remmapedRowsEvents := make(map[string]string)
	// softErrorsEvents   := make(map[string]string)
	for _, result := range checkerResults {
		if result.Status == commonCfg.StatusAbnormal {
			checkAllPassed = false
		}
		switch result.Name {
		case config.GPUPCIeACSCheckerName:
			if result.Status == commonCfg.StatusNormal {
				acsPrint = fmt.Sprintf("PCIe ACS: %sDisabled%s", Green, Reset)
				if result.Curr != "Disabled" {
					systemEvent[config.GPUPCIeACSCheckerName] = fmt.Sprintf("%s%s%s", Yellow, result.Detail, Reset)
				}
			} else {
				acsPrint = fmt.Sprintf("PCIe ACS: %sEnabled%s", Red, Reset)
				systemEvent[config.GPUPCIeACSCheckerName] = fmt.Sprintf("%sNot All PCIe ACS Are Disabled%s", Red, Reset)
			}
		case config.IOMMUCheckerName:
			if result.Status == commonCfg.StatusNormal {
				iommuPrint = fmt.Sprintf("IOMMU: %sOFF%s", Green, Reset)
			} else {
				iommuPrint = fmt.Sprintf("IOMMU: %sON%s", Red, Reset)
				systemEvent[config.IOMMUCheckerName] = fmt.Sprintf("%sIOMMU is ON%s", Red, Reset)
			}
		case config.NVFabricManagerCheckerName:
			if result.Status == commonCfg.StatusNormal {
				fabricmanagerPrint = fmt.Sprintf("FabricManager: %s%s%s", Green, result.Curr, Reset)
				if result.Curr != "Active" {
					gpuStatus[config.NVFabricManagerCheckerName] = fmt.Sprintf("%s%s%s", Yellow, result.Detail, Reset)
				}
			} else {
				fabricmanagerPrint = fmt.Sprintf("FabricManager: %sNot Active%s", Red, Reset)
				gpuStatus[config.NVFabricManagerCheckerName] = fmt.Sprintf("%sNvidia FabricManager is not active%s", Red, Reset)
			}
		case config.NvPeerMemCheckerName:
			if result.Status == commonCfg.StatusNormal {
				peermemPrint = fmt.Sprintf("nvidia_peermem: %sLoaded%s", Green, Reset)
				if result.Curr != "Loaded" {
					gpuStatus[config.NvPeerMemCheckerName] = fmt.Sprintf("%s%s%s", Yellow, result.Detail, Reset)
				}
			} else {
				peermemPrint = fmt.Sprintf("nvidia_peermem: %sNotLoaded%s", Red, Reset)
				gpuStatus[config.NvPeerMemCheckerName] = fmt.Sprintf("%snvidia_peermem module: NotLoaded%s", Red, Reset)
			}
		case config.PCIeCheckerName:
			if result.Status == commonCfg.StatusNormal {
				pcieLinkPrint = fmt.Sprintf("PCIeLink: %sOK%s", Green, Reset)
			} else {
				gpuStatus[config.PCIeCheckerName] = fmt.Sprintf("%sPCIe degradation detected:\n%s%s%s", Red, Yellow, result.Detail, Reset)
			}
		case config.HardwareCheckerName:
			if result.Status == commonCfg.StatusNormal {
				gpuStatusPrint = fmt.Sprintf("%s%d%s GPUs detected, %s%d%s GPUs used",
					Green, nvidiaInfo.DeviceCount, Reset, Green, len(nvidiaInfo.DeviceToPodMap), Reset)
			} else {
				gpuStatusPrint = fmt.Sprintf("%s%d%s GPUs detected, %s%d%s GPUs used",
					Red, nvidiaInfo.DeviceCount, Reset, Green, len(nvidiaInfo.DeviceToPodMap), Reset)
				gpuStatus[config.HardwareCheckerName] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		// case config.SoftwareCheckerName:
		case config.GpuPersistenceCheckerName:
			if result.Status == commonCfg.StatusNormal {
				persistencePrint = fmt.Sprintf("Persistence Mode: %s%s%s", Green, result.Curr, Reset)
				if result.Curr != "Enabled" {
					gpuStatus[config.GpuPersistenceCheckerName] = fmt.Sprintf("%s%s%s", Yellow, result.Detail, Reset)
				}
			} else {
				persistencePrint = fmt.Sprintf("Persistence Mode: %s%s%s", Red, result.Curr, Reset)
				gpuStatus[config.GpuPersistenceCheckerName] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		case config.GpuPStateCheckerName:
			if result.Status == commonCfg.StatusNormal {
				pstatePrint = fmt.Sprintf("PState: %s%s%s", Green, result.Curr, Reset)
			} else {
				pstatePrint = fmt.Sprintf("PState: %s%s%s", Red, result.Curr, Reset)
				gpuStatus[config.GpuPStateCheckerName] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		case config.NvlinkCheckerName:
			if result.Status == commonCfg.StatusNormal {
				nvlinkPrint = fmt.Sprintf("NVLink: %s%s%s", Green, result.Curr, Reset)
			} else {
				nvlinkPrint = fmt.Sprintf("NVLink: %s%s%s", Green, result.Curr, Reset)
				gpuStatus[config.NvlinkCheckerName] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		case config.AppClocksCheckerName:
			if result.Status == commonCfg.StatusNormal {
				// gpuStatus[config.AppClocksCheckerName] = fmt.Sprintf("%sGPU application clocks: Set to maximum%s", Green, Reset)
			} else {
				gpuStatus[config.AppClocksCheckerName] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
			}
		case config.ClockEventsCheckerName:
			if result.Status == commonCfg.StatusNormal {
				clockEvents["Thermal"] = fmt.Sprintf("%sNo HW Thermal Slowdown Found%s", Green, Reset)
				clockEvents["PowerBrake"] = fmt.Sprintf("%sNo HW Power Brake Slowdown Found%s", Green, Reset)
			} else {
				clockEvents["Thermal"] = fmt.Sprintf("%sHW Thermal Slowdown Found%s", Red, Reset)
				clockEvents["PowerBrake"] = fmt.Sprintf("%sHW Power Brake Slowdown Found%s", Red, Reset)
			}
		case config.SRAMAggUncorrectableCheckerName:
			if result.Status == commonCfg.StatusNormal {
				eccEvents[config.SRAMAggUncorrectableCheckerName] = fmt.Sprintf("%sNo SRAM Agg Uncorrectable Found%s", Green, Reset)
			} else {
				eccEvents[config.SRAMAggUncorrectableCheckerName] = fmt.Sprintf("%sSRAM Agg Uncorrectable Found%s", Red, Reset)
			}
		case config.SRAMHighcorrectableCheckerName:
			if result.Status == commonCfg.StatusNormal {
				eccEvents[config.SRAMHighcorrectableCheckerName] = fmt.Sprintf("%sNo SRAM High Correctable Found%s", Green, Reset)
			} else {
				eccEvents[config.SRAMHighcorrectableCheckerName] = fmt.Sprintf("%sSRAM High Correctable Found%s", Red, Reset)
			}
		case config.SRAMVolatileUncorrectableCheckerName:
			if result.Status == commonCfg.StatusNormal {
				eccEvents[config.SRAMVolatileUncorrectableCheckerName] = fmt.Sprintf("%sNo SRAM Volatile Uncorrectable Found%s", Green, Reset)
			} else {
				eccEvents[config.SRAMVolatileUncorrectableCheckerName] = fmt.Sprintf("%sSRAM Volatile Uncorrectable Found%s", Red, Reset)
			}
		case config.RemmapedRowsFailureCheckerName:
			if result.Status == commonCfg.StatusNormal {
				remmapedRowsEvents[config.RemmapedRowsFailureCheckerName] = fmt.Sprintf("%sNo Remmaped Rows Failure Found%s", Green, Reset)
			} else {
				remmapedRowsEvents[config.RemmapedRowsFailureCheckerName] = fmt.Sprintf("%sRemmaped Rows Failure Found%s", Red, Reset)
			}
		case config.RemmapedRowsUncorrectableCheckerName:
			if result.Status == commonCfg.StatusNormal {
				remmapedRowsEvents[config.RemmapedRowsUncorrectableCheckerName] = fmt.Sprintf("%sNo Remmaped Rows Uncorrectable Found%s", Green, Reset)
			} else {
				remmapedRowsEvents[config.RemmapedRowsUncorrectableCheckerName] = fmt.Sprintf("%sRemmaped Rows Uncorrectable Found%s", Red, Reset)
			}
		case config.RemmapedRowsPendingCheckerName:
			if result.Status == commonCfg.StatusNormal {
				remmapedRowsEvents[config.RemmapedRowsPendingCheckerName] = fmt.Sprintf("%sNo Remmaped Rows Pending Found%s", Green, Reset)
			} else {
				remmapedRowsEvents[config.RemmapedRowsPendingCheckerName] = fmt.Sprintf("%sRemmaped Rows Pending Found%s", Red, Reset)
			}
		}
	}
	if summaryPrint {
		utils.PrintTitle("NVIDIA GPUs", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 40
		if err == nil {
			printInterval = termWidth / 3
		}

		driverPrint = fmt.Sprintf("Driver Version: %s%s%s", Green, nvidiaInfo.SoftwareInfo.DriverVersion, Reset)
		cudaVersionPrint = fmt.Sprintf("CUDA Version: %s%s%s", Green, nvidiaInfo.SoftwareInfo.CUDAVersion, Reset)
		gpuNumPrint := "GPU NUMs:"
		fmt.Printf("%s\n", nvidiaInfo.DevicesInfo[0].Name)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, driverPrint, printInterval, iommuPrint, printInterval, persistencePrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, cudaVersionPrint, printInterval, acsPrint, printInterval, pstatePrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval-PadLen, gpuNumPrint, printInterval, peermemPrint, printInterval, nvlinkPrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval+PadLen, gpuStatusPrint, printInterval, fabricmanagerPrint, printInterval, pcieLinkPrint)
		fmt.Println()
	}
	if len(systemEvent) > 0 {
		fmt.Println("System Settings and Status:")
		for _, v := range systemEvent {
			fmt.Printf("\t%s\n", v)
		}
	}
	if len(gpuStatus) > 0 {
		fmt.Println("NVIDIA GPU:")
		for _, v := range gpuStatus {
			fmt.Printf("\t%s\n", v)
		}
	}
	fmt.Println("Clock Events:")
	for _, v := range clockEvents {
		fmt.Printf("\t%s\n", v)
	}
	fmt.Println("Memory ECC:")
	for _, v := range eccEvents {
		fmt.Printf("\t%s\n", v)
	}
	fmt.Println("Remapped Rows:")
	for _, v := range remmapedRowsEvents {
		fmt.Printf("\t%s\n", v)
	}
	return checkAllPassed
}
