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
	"github.com/scitix/sichek/components/hang"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewHangCommand() *cobra.Command {
	hangCmd := &cobra.Command{
		Use:     "hang",
		Aliases: []string{"h"},
		Short:   "Perform Hang check",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			verbos, err := cmd.Flags().GetBool("verbos")
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the verbose: %v", err)
			}
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				defer func() {
					logrus.WithField("component", "Hang").Info("Run Hang Cmd context canceled")
					cancel()
				}()
			}
			cfgFile, err := cmd.Flags().GetString("cfg")
			if err != nil {
				logrus.WithField("component", "Hang").Error(err)
				return
			} else {
				logrus.WithField("component", "Hang").Infof("load cfg file:%s", cfgFile)
			}

			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				logrus.WithField("component", "Hang").Error(err)
				return
			} else {
				logrus.WithField("component", "Hang").Infof("load spec file:%s", specFile)
			}
			component, err := hang.NewComponent(cfgFile, specFile)
			if err != nil {
				logrus.WithField("components", "Hang").Error("fail to Create Hang Components")
				return
			}

			subctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()

			var wg sync.WaitGroup
			wg.Add(1)
			go func(ctx context.Context) {
				defer func() {
					if err := recover(); err != nil {
						logrus.WithField("component", "Hang").Errorf("recover panic NewHangCommand() err: %v", err)
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
							logrus.WithField("component", "Hang").Errorf("analyze hang failed: %v", err)
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
				logrus.WithField("component", "Hang").Errorf("analyze hang failed: %v", err)
				return
			}

			logrus.WithField("component", "Hang").Infof("Hang analysis result: %s\n", common.ToString(result))
			info, err := component.LastInfo()
			if err != nil {
				logrus.WithField("component", "all").Errorf("get to ge the LastInfo: %v", err)
			}
			pass := component.PrintInfo(info, result, true)
			StatusMutex.Lock()
			ComponentStatuses[consts.ComponentNameHang] = pass
			StatusMutex.Unlock()
		},
	}

	hangCmd.Flags().StringP("cfg", "c", "", "Path to the Hang Cfg file")
	hangCmd.Flags().StringP("spec", "s", "", "Path to the GPU spec file")
	hangCmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")

	return hangCmd
}
