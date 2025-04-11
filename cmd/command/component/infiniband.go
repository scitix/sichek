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
	"github.com/scitix/sichek/components/infiniband"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewInfinibandCmd() *cobra.Command {
	infinibandCmd := &cobra.Command{
		Use:     "infiniband",
		Aliases: []string{"i"},
		Short:   "Perform Infiniband check - related operations",
		Long:    "Used to perform specific Infiniband - related operations, with specific functions to be expanded",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), CmdTimeout)

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the verbose: %v", err)
			}
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("component", "infiniband").Info("Run infiniband Cmd context canceled")
					cancel()
				}()
			}

			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "infiniband").Error(err)
				return
			} else {
				logrus.WithField("component", "infiniband").Info("load default cfg...")
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("components", "infiniband").Error(err)
			} else {
				if specFile != "" {
					logrus.WithField("components", "infiniband").Info("load specFile: " + specFile)
				} else {
					logrus.WithField("components", "infiniband").Info("load default specFile...")
				}
			}

			ignoredCheckersStr, err := cmd.Flags().GetString("ignored-checkers")
			if err != nil {
				logrus.WithField("components", "infiniband").Error(err)
			} else {
				logrus.WithField("components", "infiniband").Info("ignore checkers", ignoredCheckersStr)
			}
			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}
			component, err := infiniband.NewInfinibandComponent(cfgFile, specFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", component.Name()).Error("fail to Create New Infiniband Components")
				return
			}

			result, err := common.RunHealthCheckWithTimeout(ctx, CmdTimeout, component.Name(), component.HealthCheck)
			if err != nil {
				logrus.WithField("component", component.Name()).Error(err)
				return
			}

			// logrus.WithField("component", component.Name()).Infof("Analysis Result: %s\n", common.ToString(result))
			info, err := component.LastInfo()
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := PrintInfinibandInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameInfiniband] = pass
			StatusMutex.Unlock()
		},
	}

	infinibandCmd.Flags().StringP("cfg", "c", "", "Path to the Infinibnad Cfg")
	infinibandCmd.Flags().StringP("spec", "s", "", "Path to the Infinibnad Spec")
	infinibandCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	infinibandCmd.Flags().StringP("ignored-checkers", "i", "", "Ignored checkers")
	return infinibandCmd
}

func PrintInfinibandInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true

	ibInfo, ok := info.(*collector.InfinibandInfo)
	if !ok {
		logrus.WithField("component", "infiniband").Errorf("invalid data type, expected InfinibandInfo")
		return false
	}

	checkerResults := result.Checkers
	ibControllersPrintColor := Green
	// PerformancePrint := "Performance: "

	var (
		ibKmodPrint      string
		ofedVersionPrint string
		fwVersionPrint   string
		ibPortSpeedPrint string
		phyStatPrint     string
		ibStatePrint     string
		pcieLinkPrint    string
		// throughPrint        string
		// latencyPrint     string
	)
	pcieGen := ""
	pcieWidth := ""

	infinibandEvents := make(map[string]string)
	ofedVersionPrint = fmt.Sprintf("OFED Version: %s%s%s", Green, ibInfo.IBSoftWareInfo.OFEDVer, Reset)

	logrus.Infof("checkerResults: %v", common.ToString(checkerResults))

	for _, result := range checkerResults {
		statusColor := Green
		if result.Status != consts.StatusNormal {
			statusColor = Red
			infinibandEvents[result.Name] = fmt.Sprintf("%s%s%s", statusColor, result.Detail, Reset)
			checkAllPassed = false
		}

		switch result.Name {
		case config.ChekIBOFED:
			ofedVersionPrint = fmt.Sprintf("OFED Version: %s%s%s", statusColor, result.Curr, Reset)
		case config.CheckIBKmod:
			ibKmodPrint = fmt.Sprintf("Infiniband Kmod: %s%s%s", statusColor, "Loaded", Reset)
			if result.Status != consts.StatusNormal {
				ibKmodPrint = fmt.Sprintf("Infiniband Kmod: %s%s%s", statusColor, "Not Loaded Correctly", Reset)
			}
		case config.ChekIBFW:
			fwVersion := extractAndDeduplicate(result.Curr)
			fwVersionPrint = fmt.Sprintf("FW Version: %s%s%s", statusColor, fwVersion, Reset)
		case config.ChekIBPortSpeed:
			portSpeed := extractAndDeduplicate(result.Curr)
			ibPortSpeedPrint = fmt.Sprintf("IB Port Speed: %s%s%s", statusColor, portSpeed, Reset)
		case config.ChekIBPhyState:
			phyState := "LinkUp"
			if result.Status != consts.StatusNormal {
				phyState = "Not All LinkUp"
			}
			phyStatPrint = fmt.Sprintf("Phy State: %s%s%s", statusColor, phyState, Reset)
		case config.ChekIBState:
			ibState := "Active"
			if result.Status != consts.StatusNormal {
				ibState = "Not All Active"
			}
			ibStatePrint = fmt.Sprintf("IB State: %s%s%s", statusColor, ibState, Reset)
		case config.CheckPCIESpeed:
			pcieGen = fmt.Sprintf("%s%s%s", statusColor, extractAndDeduplicate(result.Curr), Reset)
		case config.CheckPCIEWidth:
			pcieWidth = fmt.Sprintf("%s%s%s", statusColor, extractAndDeduplicate(result.Curr), Reset)
		case config.CheckIBDevs:
			ibControllersPrintColor = statusColor
		}
	}
	if pcieGen != "" && pcieWidth != "" {
		pcieLinkPrint = fmt.Sprintf("PCIe Link: %s%s (x%s)%s", Green, pcieGen, pcieWidth, Reset)
	} else {
		pcieLinkPrint = fmt.Sprintf("PCIe Link: %sError Detected%s", Red, Reset)
	}

	ibControllersPrint := fmt.Sprintf("Host Channel Adaptor: %s", ibControllersPrintColor)
	for _, hwInfo := range ibInfo.IBHardWareInfo {
		ibControllersPrint += fmt.Sprintf("%s(%s), ", hwInfo.IBDev, hwInfo.NetDev)
	}

	ibControllersPrint = strings.TrimSuffix(ibControllersPrint, ", ")
	ibControllersPrint += Reset

	if summaryPrint {
		utils.PrintTitle("infiniband", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 60
		if err == nil {
			printInterval = termWidth / 3
		}
		if printInterval < len(ofedVersionPrint) {
			printInterval = len(ofedVersionPrint) + 2
		}
		fmt.Printf("%-*s\n", printInterval, ibControllersPrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, ibKmodPrint, printInterval, phyStatPrint, printInterval, "")          //, PerformancePrint)
		fmt.Printf("%-*s%-*s\t%-*s\n", printInterval, ofedVersionPrint, printInterval, ibStatePrint, printInterval, "")   //, "Throughput: TBD")
		fmt.Printf("%-*s%-*s\t%-*s\n", printInterval, fwVersionPrint, printInterval, ibPortSpeedPrint, printInterval, "") //, "Latency: TBD")
		fmt.Printf("%-*s%-*s\n", printInterval, Green+""+Reset, printInterval, pcieLinkPrint)
	}

	fmt.Println("Errors Events:")

	if len(infinibandEvents) == 0 {
		fmt.Printf("\t%sNo Infiniband Events Detected%s\n", Green, Reset)
	} else {
		for _, event := range infinibandEvents {
			fmt.Printf("\t%s\n", event)
		}
	}
	return checkAllPassed
}

func extractAndDeduplicate(curr string) string {
	// Split the string by ';'
	values := strings.Split(curr, ",")

	// Use a map to store unique values
	uniqueValues := make(map[string]struct{})
	for _, value := range values {
		if value != "" { // Ignore empty strings
			uniqueValues[value] = struct{}{}
		}
	}

	// Collect keys from the map into a slice
	result := make([]string, 0, len(uniqueValues))
	for key := range uniqueValues {
		result = append(result, key)
	}

	// Join the unique values back into a single string
	return strings.Join(result, ",")
}
