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
package eventfilter

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/scitix/sichek/components/common"
	"github.com/sirupsen/logrus"
)

type Command struct {
	Command string
	Args    []string
	CmdDesc string
}

type CommandFilter struct {
	Regex      []*RegexFilter
	Commands   []*Command
	CacheLineN int64

	LogFileName []string
	EventFilter *EventFilter
}

func NewCommandFilter(cmd []string, rules map[string]*common.EventRuleConfig, cacheLine int64) (*CommandFilter, error) {
	var res CommandFilter
	res.Regex = make([]*RegexFilter, 0)
	res.Commands = make([]*Command, 0)
	res.CacheLineN = cacheLine
	if len(rules) == 0 {
		logrus.WithField("CommandFilter", "NewCommandFilter").Error("no rules provided")
		return nil, fmt.Errorf("no rules provided in CommandFilter new")
	}
	if len(cmd) == 0 {
		logrus.WithField("CommandFilter", res.Regex).Error("failed to save cmd[%ld] which is empty")
		return nil, fmt.Errorf("empty cmd in CommandFilter new")
	} else {
		res.Commands = append(res.Commands, NewCommand(cmd[0], cmd[1:]...))
	}

	logFileName := "/tmp/sichek." + cmd[0] + ".log"
	for _, rule := range rules {
		rule.LogFile = logFileName
	}
	res.LogFileName = append(res.LogFileName, logFileName)

	var err error
	res.EventFilter, err = NewEventFilter("dmesg", rules, res.CacheLineN, 0)
	if err != nil {
		logrus.WithField("CommandFilter", res.Regex).Error("failed to create fileFilter")
		return nil, err
	}

	return &res, nil
}

func NewCommand(command string, args ...string) *Command {
	var res Command
	res.Command = command
	res.Args = args

	res.CmdDesc = res.Command + " "
	for j := 0; j < len(res.Args); j++ {
		res.CmdDesc = res.CmdDesc + res.Args[j] + " "
	}

	return &res
}

func (f *CommandFilter) Check() *common.Result {
	for k := 0; k < len(f.Commands); k++ {
		fd, err := os.OpenFile(f.LogFileName[k], os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			logrus.WithField("CommandFilter", f.Regex).WithField("LogFile", f.LogFileName[k]).Error("failed to open file")
		}
		defer func(fd *os.File) {
			err := fd.Close()
			if err != nil {
				logrus.WithField("CommandFilter", f.Regex).WithField("LogFile", f.LogFileName[k]).Error(err)
			}
		}(fd)

		command := f.Commands[k]
		cmd := exec.Command(command.Command, command.Args...)
		cmd.Stdout = fd // be careful cmd output is append to file
		cmd.Stderr = fd

		if err := cmd.Run(); err != nil {
			logrus.WithField("Command", command.CmdDesc).WithError(err).Error("failed to run cmd")
			return nil
		}
	}
	return f.EventFilter.Check()
}

func (f *CommandFilter) Close() bool {
	return f.EventFilter.Close()
}
