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
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewGpuEventsCommand() *cobra.Command {
	gpuEventsCmd := &cobra.Command{
		Use:     "gpuevents",
		Aliases: []string{"h"},
		Short:   "Perform costom gpu events check",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			verbos, err := cmd.Flags().GetBool("verbos")
			if err != nil {
				logrus.WithField("component", "gpuevents").Errorf("get to ge the verbose: %v", err)
			}
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("component", "gpuevents").Info("Run gpuevents Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "gpuevents").Error(err)
				return
			} else {
				logrus.WithField("component", "gpuevents").Infof("load cfg file:%s", cfgFile)
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("component", "gpuevents").Error(err)
				return
			} else {
				logrus.WithField("component", "gpuevents").Infof("load spec file:%s", specFile)
			}
			// component, err := gpuevents.NewComponent(cfgFile, specFile)
			component, err := NewComponent(consts.ComponentNameGpuEvents, cfgFile, specFile, nil)
			if err != nil {
				logrus.WithField("component", "gpuevents").Error("fail to Create gpuevents Components")
				return
			}

			subctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()

			var wg sync.WaitGroup
			wg.Add(1)
			go func(ctx context.Context) {
				defer func() {
					if err := recover(); err != nil {
						logrus.WithField("component", "gpuevents").Errorf("recover panic NewGpuEventsCommand() err: %v", err)
					}
				}()
				defer wg.Done()
				select {
				case <-ctx.Done():
					fmt.Println("Timeout! Task canceled.")
					return
				default:
					fmt.Println("Task running...")
					begin := time.Now()
					for time.Since(begin).Seconds() < 720 {
						_, err := component.HealthCheck(ctx)
						if err != nil {
							logrus.WithField("component", "gpuevents").Errorf("analyze gpuevents failed: %v", err)
							return
						}
						time.Sleep(10 * time.Second)
					}
					fmt.Println("...Task finished")
				}
			}(subctx)
			wg.Wait()

			result, err := common.RunHealthCheckWithTimeout(ctx, consts.CmdTimeout, component.Name(), component.HealthCheck)
			if err != nil {
				logrus.WithField("component", "gpuevents").Errorf("analyze gpuevents failed: %v", err)
				return
			}

			logrus.WithField("component", "gpuevents").Infof("gpuevents analysis result: %s\n", common.ToString(result))
			info, err := component.LastInfo()
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := component.PrintInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameGpuEvents] = pass
			StatusMutex.Unlock()
		},
	}

	gpuEventsCmd.Flags().StringP("cfg", "c", "", "Path to the gpuevents Cfg file")
	gpuEventsCmd.Flags().StringP("spec", "s", "", "Path to the GPU spec file")
	gpuEventsCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return gpuEventsCmd
}
