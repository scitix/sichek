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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/config"
	ethernetcfg "github.com/scitix/sichek/config/ethernet"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewEthernetCmd() *cobra.Command {
	ethernetCmd := &cobra.Command{
		Use:     "ethernet",
		Aliases: []string{"e"},
		Short:   "Perform ethernet - related operations",
		Long:    "Used to perform specific ethernet - related operations, with specific functions to be expanded",
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
					logrus.WithField("component", "ethernet").Info("Run Ethernet HealthCheck Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "ethernet").Error(err)
			} else {
				logrus.WithField("component", "ethernet").Info("load default cfg...")
			}
			cfg, err := config.LoadComponentConfig(cfgFile, "")
			if err != nil {
				logrus.WithField("component", "ethernet").Errorf("create ethernet component failed: %v", err)
				return
			}
			component, err := ethernet.NewEthernetComponent(cfg)
			if err != nil {
				logrus.WithField("component", "ethernet").Error("fail to Create New Infiniband Components")
			}

			result, err := component.HealthCheck(ctx)

			if err != nil {
				logrus.WithField("component", component.Name()).Error(err)
				return
			}

			logrus.WithField("component", "ethernet").Infof("Analysis Result: %s\n", common.ToString(result))
			info, err := component.LastInfo(ctx)
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := PrintEthernetInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameEthernet] = pass
			StatusMutex.Unlock()
		},
	}

	ethernetCmd.Flags().StringP("cfg", "c", "", "Path to the Infinibnad Cfg")
	ethernetCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return ethernetCmd
}

func PrintEthernetInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	ethInfo, ok := info.(*collector.EthernetInfo)
	if !ok {
		logrus.WithField("component", "ethernet").Errorf("invalid data type, expected EthernetInfo")
		return false
	}

	ethControllersPrint := "Ethernet Nic: "
	var phyStatPrint string

	ethernetEvents := make(map[string]string)
	for _, ethDev := range ethInfo.EthDevs {
		ethControllersPrint += fmt.Sprintf("%s, ", ethDev)
	}
	ethControllersPrint = ethControllersPrint[:len(ethControllersPrint)-2]

	checkerResults := result.Checkers
	for _, result := range checkerResults {
		switch result.Name {
		case ethernetcfg.ChekEthPhyState:
			if result.Status == consts.StatusNormal {
				phyStatPrint = fmt.Sprintf("Phy State: %sLinkUp%s", Green, Reset)
			} else {
				phyStatPrint = fmt.Sprintf("Phy State: %sLinkDown%s", Red, Reset)
				ethernetEvents["phy_state"] = fmt.Sprintf("%s%s%s", Red, result.Detail, Reset)
				checkAllPassed = false
			}
		}
	}

	if summaryPrint {
		utils.PrintTitle("Ethernet", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 40
		if err == nil {
			printInterval = termWidth / 3
		}

		fmt.Printf("%-*s%-*s\n", printInterval, ethControllersPrint, printInterval, phyStatPrint)
		fmt.Println()
	}
	fmt.Println("Errors Events:")
	if len(ethernetEvents) == 0 {
		fmt.Println("\tNo ethernet Events Detected")
		return checkAllPassed
	}
	fmt.Printf("%16s : %-10s\n", "checkItems", "checkDetail")
	for item, v := range ethernetEvents {
		fmt.Printf("%16s : %-10s\n", item, v)
	}
	return checkAllPassed
}
