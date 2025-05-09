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
package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/scitix/sichek/pkg/systemd"

	sd "github.com/coreos/go-systemd/v22/daemon"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var AllowedSignals = []os.Signal{
	unix.SIGTERM,
	unix.SIGINT,
	unix.SIGUSR1,
	unix.SIGPIPE,
}

func HandleSignals(cancel context.CancelFunc, signals chan os.Signal, serverC chan Service) chan struct{} {
	done := make(chan struct{}, 1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("[HandleSignals] panic err is %s\n", err)
			}
		}()
		var server Service
		for {
			select {
			case s := <-serverC:
				server = s
			case s := <-signals:

				// Do not print message when deailing with SIGPIPE, which may cause
				// nested signals and consume lots of cpu bandwidth.
				if s == unix.SIGPIPE {
					continue
				}

				logrus.Debugf("received signal: %v", s)
				switch s {
				case unix.SIGUSR1:
					dumpStacks(true)
				default:
					cancel()

					if exist, _ := systemd.SystemctlExists(); exist {
						if err := NotifyStopping(); err != nil {
							logrus.Error("notify stopping failed")
						}
					}

					if server == nil {
						close(done)
						return
					}

					err := server.Stop()
					if err != nil {
						logrus.Errorf("server.Stop: %v", err)
					}
					close(done)
					return
				}
			}
		}
	}()
	return done
}

// NotifyReady notifies systemd that the daemon is ready to serve requests
func NotifyReady() error {
	return sdNotify(sd.SdNotifyReady)
}

// NotifyStopping notifies systemd that the daemon is about to be stopped
func NotifyStopping() error {
	return sdNotify(sd.SdNotifyStopping)
}

func sdNotify(state string) error {
	notified, err := sd.SdNotify(false, state)
	logrus.Debugf("sd notification: %v %v %v", state, notified, err)
	return err
}

func dumpStacks(writeToFile bool) {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	logrus.Debugf("=== BEGIN goroutine stack dump ===\n%s\n=== END goroutine stack dump ===", buf)

	if writeToFile {
		// Also write to file to aid gathering diagnostics
		name := filepath.Join(os.TempDir(), fmt.Sprintf("sichek.%d.stacks.log", os.Getpid()))
		f, err := os.Create(name)
		if err != nil {
			return
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				logrus.Errorf("failed to close file %s", name)
			}
		}(f)
		_, _ = f.WriteString(string(buf))
		logrus.Debugf("goroutine stack dump written to %s", name)
	}
}
