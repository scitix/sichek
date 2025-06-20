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
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

var ID2Filter map[string]*EventFilter

type EventFilter struct {
	ID           string
	RegexEntries []*RegexEntry
	FileEntryMap map[string]*FileEntry
	CacheLineN   int64
}

type RegexEntry struct {
	Rule  *common.EventRuleConfig
	Regex *RegexFilter
	Files []string
}

type FileEntry struct {
	FileName     string
	Loader       *FileLoader
	CheckPos     int64
	CheckLinePos int64
}

func NewEventFilter(id string, rules map[string]*common.EventRuleConfig, cacheLine int64, skipPercent int64) (*EventFilter, error) {
	ef := &EventFilter{
		ID:           id,
		RegexEntries: []*RegexEntry{},
		FileEntryMap: make(map[string]*FileEntry),
		CacheLineN:   cacheLine,
	}
	for _, rule := range rules {
		if rule.Name == "" || rule.Regexp == "" || rule.LogFile == "" {
			logrus.WithField("EventFilter", "NewEventFilter").Errorf("invalid rule: %+v", rule)
			continue
		}
		// Create a new RegexFilter for each event rule
		regex := NewRegexFilter(rule.Name, rule.Regexp)
		if err := regex.Compile(); err != nil {
			logrus.WithField("EventFilter", "NewFileFilterSkip").WithError(err).Errorf("Failed to compile regex for event rule: %s", rule.Name)
			continue
		}
		// support multiple file names with comma separation
		validfiles := checkFile(rule.LogFile)
		if len(validfiles) == 0 {
			logrus.WithField("EventFilter", "NewEventFilter").Errorf("No valid log files found for event rule: %s", rule.Name)
			continue
		}
		entry := &RegexEntry{Rule: rule, Regex: regex, Files: validfiles}
		ef.RegexEntries = append(ef.RegexEntries, entry)
		for _, fileName := range validfiles {
			if _, ok := ef.FileEntryMap[fileName]; !ok {
				loader, err := NewFileLoader(fileName, cacheLine, skipPercent)
				if loader == nil || err != nil {
					logrus.WithField("EventFilter", "NewEventFilter").WithError(err).Errorf("Failed to create file loader for file: %s", fileName)
					continue
				}
				ef.FileEntryMap[fileName] = &FileEntry{FileName: fileName, Loader: loader, CheckPos: 0, CheckLinePos: 0}
			}
		}
	}
	if len(ef.RegexEntries) == 0 {
		return nil, errors.New("no valid regex rules found")
	}
	return ef, nil
}

func (f *EventFilter) Check() *common.Result {
	resultMap := make(map[string]*common.CheckerResult)

	for fname, entry := range f.FileEntryMap {
		loader := entry.Loader
		if loader.LogLineNum-entry.CheckLinePos > loader.CacheNum {
			entry.CheckLinePos = loader.LogLineNum - loader.CacheNum
		}

		for entry.CheckLinePos < loader.LogLineNum {
			line := loader.CachedLines[entry.CheckLinePos%loader.CacheNum]
			entry.CheckLinePos++
			f.matchLineAndUpdateResult(fname, line, resultMap)
		}
	}

	result := &common.Result{
		Time:     time.Now(),
		Status:   consts.StatusNormal,
		Level:    consts.LevelInfo,
		Checkers: make([]*common.CheckerResult, 0),
	}
	for _, checkItem := range resultMap {
		if checkItem != nil {
			result.Checkers = append(result.Checkers, checkItem)
			if checkItem.Status == consts.StatusAbnormal {
				logrus.WithField("component", "filefilter").Warnf("Abnormal check result: %s, %s", checkItem.Name, checkItem.Detail)
				result.Status = consts.StatusAbnormal
				if consts.LevelPriority[result.Level] > consts.LevelPriority[checkItem.Level] {
					result.Level = checkItem.Level
				}
			}
		}
	}
	return result
}

func (f *EventFilter) matchLineAndUpdateResult(fileName string, line string, resultMap map[string]*common.CheckerResult) {
	for _, entry := range f.RegexEntries {
		if !contains(entry.Files, fileName) {
			continue
		}
		if entry.Regex.MatchOneLine(line) {
			rule := entry.Rule
			name := rule.Name
			res, exists := resultMap[name]
			if !exists {
				res = &common.CheckerResult{
					Name:        name,
					Description: rule.Description,
					Curr:        "1",
					Device:      fileName,
					Level:       rule.Level,
					ErrorName:   name,
					Suggestion:  rule.Suggestion,
					Detail:      line,
				}
				resultMap[name] = res
			} else {
				curr, _ := strconv.Atoi(res.Curr)
				curr++
				res.Curr = strconv.Itoa(curr)
				if !strings.Contains(res.Device, fileName) {
					res.Device += "," + fileName
				}
				if curr < 3 {
					res.Detail += "\n" + line
				}
			}
			break // one line can match only one regex
		}
	}
}

func (f *EventFilter) Close() bool {
	res := true
	for _, entry := range f.FileEntryMap {
		if entry.Loader.Close() != nil {
			logrus.WithField("EventFilter", entry.FileName).Error("failed to close fileloader")
			res = false
		}
	}
	return res
}

func checkFile(filelist string) []string {
	var allFiles []string
	files := strings.Split(filelist, ",")
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		allFiles = append(allFiles, f)
	}
	return allFiles
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
