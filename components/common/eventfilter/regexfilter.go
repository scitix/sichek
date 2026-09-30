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
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"
)

var Name2RegexFilter sync.Map

type RegexFilter struct {
	Name            string
	RegexExpression string
	RegexObj        *regexp.Regexp

	hasCompile bool
}

func NewRegexFilter(name string, regexpression string) *RegexFilter {
	value, exists := Name2RegexFilter.Load(regexpression)
	if exists {
		logrus.WithField("RegexFilter", regexpression).Debug("failed to new regex filter, because it has existed")
		if res, ok := value.(*RegexFilter); ok {
			return res
		} else {
			logrus.WithField("RegexFilter", regexpression).Warn("prev value in regex filter map type isnot *RegexFilter")
		}
	}

	res := &RegexFilter{
		Name:            name,
		RegexExpression: regexpression,
		hasCompile:      false,
	}
	Name2RegexFilter.Store(regexpression, res)
	return res
}

func (f *RegexFilter) Compile() error {
	if f.hasCompile {
		return nil
	}

	res, err := regexp.Compile(f.RegexExpression)
	if err != nil {
		logrus.WithField("filter", f.RegexExpression).Error("failed to compile RegexFilter")
		return err
	}
	f.RegexObj = res
	f.hasCompile = true
	return nil
}

func (f *RegexFilter) MatchOneLine(line string) bool {
	if f.RegexObj != nil && f.RegexObj.MatchString(line) {
		return true
	}
	return false
}

func (f *RegexFilter) MatchBytes(line []byte) bool {
	if f.RegexObj != nil && f.RegexObj.Match(line) {
		return true
	}
	return false
}
