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
package filter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var ID2Filter map[string]*FileFilter

type FileFilter struct {
	Regex      []*RegexFilter
	CacheLineN int64

	FileNames        []string
	FileLoaders      []*FileLoader
	FileCheckPos     []int64
	FileCheckLinePos []int64
}

func NewFileFilter(regexpName []string, regexps []string, filesName []string, cacheLine int64) (*FileFilter, error) {
	return NewFileFilterSkip(regexpName, regexps, filesName, cacheLine, 100)
}

func NewFileFilterWithReg(regexs []*RegexFilter, filesName []string, cacheLineN int64) (*FileFilter, error) {
	return NewFileFilterWithRegSkip(regexs, filesName, cacheLineN, 100)
}

func NewFileFilterSkip(regexpName []string, regexps []string, filesName []string, cacheLine int64, skipPercent int64) (*FileFilter, error) {
	if len(regexpName) != len(regexps) {
		logrus.Error("wrong input, u need spesify a name for each regexps")
		return nil, fmt.Errorf("no Name specified for regexp")
	}

	var regexs []*RegexFilter
	for i := 0; i < len(regexps); i++ {
		regexs = append(regexs, NewRegexFilter(regexpName[i], regexps[i]))
	}
	return NewFileFilterWithRegSkip(regexs, filesName, cacheLine, skipPercent)
}

func NewFileFilterWithRegSkip(regexs []*RegexFilter, filesName []string, cacheLineN int64, skipPercent int64) (*FileFilter, error) {
	var res FileFilter
	res.CacheLineN = cacheLineN
	res.Regex = regexs
	for i := 0; i < len(res.Regex); i++ {
		if err := res.Regex[i].Compile(); err != nil {
			return nil, err
		}
	}

	for i := 0; i < len(filesName); i++ {
		res.AppendFile(filesName[i], res.CacheLineN, skipPercent)
	}
	return &res, nil
}

func (f *FileFilter) CheckFileCache() []FilterResult {
	var res []FilterResult
	for i := 0; i < len(f.FileLoaders); i++ {
		_, err := f.FileLoaders[i].Load()
		if err != nil {
			logrus.WithField("FileFilter", "CheckFileCache").WithField("f.FileLoaders[i].Load() err: ", err)
		}
	}

	for i := 0; i < len(f.FileLoaders); i++ {
		fileLoader := f.FileLoaders[i]
		if fileLoader.LogLineNum-f.FileCheckLinePos[i] > fileLoader.CacheNum {
			f.FileCheckLinePos[i] = fileLoader.LogLineNum - fileLoader.CacheNum
		}

		for f.FileCheckLinePos[i] < fileLoader.LogLineNum {
			for j := 0; j < len(f.Regex); j++ {
				if f.Regex[j].MatchOneLine(fileLoader.CachedLines[f.FileCheckLinePos[i]%fileLoader.CacheNum]) {
					res = append(res, FilterResult{
						Regex:    f.Regex[j].RegexExpression,
						Name:     f.Regex[j].Name,
						FileName: fileLoader.Name,
						Line:     fileLoader.CachedLines[f.FileCheckLinePos[i]%fileLoader.CacheNum],
					})
				}
			}
			f.FileCheckPos[i] += int64(len(fileLoader.CachedLines[f.FileCheckLinePos[i]%fileLoader.CacheNum]))
			f.FileCheckLinePos[i]++
		}
	}
	return res
}

func (f *FileFilter) Check() []FilterResult {
	var res []FilterResult

	for i := 0; i < len(f.FileLoaders); i++ {
		fileLoader := f.FileLoaders[i]

		for {
			newLines, err := fileLoader.GetLines(f.FileCheckPos[i])
			if err != nil {
				logrus.WithField("FileFilter", fileLoader.Name).WithError(err).Error("failed to get file's new line")
			}
			if len(newLines) == 0 {
				break
			}

			for k := 0; k < len(newLines); k++ {
				for j := 0; j < len(f.Regex); j++ {
					if f.Regex[j].MatchOneLine(newLines[k]) {
						res = append(res, FilterResult{
							Regex:    f.Regex[j].RegexExpression,
							Name:     f.Regex[j].Name,
							FileName: fileLoader.Name,
							Line:     newLines[k],
						})
					}
				}
				f.FileCheckPos[i] += int64(len(newLines[k]))
				f.FileCheckLinePos[i]++
			}
		}

	}
	return res
}

func (f *FileFilter) AppendFile(fileName string, cacheNum int64, skipPercent int64) bool {
	fileLoader := NewFileLoader(fileName, cacheNum, skipPercent)
	if fileLoader == nil {
		return false
	}

	f.FileNames = append(f.FileNames, fileName)
	f.FileLoaders = append(f.FileLoaders, fileLoader)
	f.FileCheckPos = append(f.FileCheckPos, f.FileLoaders[len(f.FileLoaders)-1].Pos)
	f.FileCheckLinePos = append(f.FileCheckLinePos, 0)
	return true
}

func (f *FileFilter) Close() bool {
	res := true
	for i := 0; i < len(f.FileLoaders); i++ {
		ok := f.FileLoaders[i].Close()
		if !ok {
			logrus.WithField("FileFilter", f.FileLoaders[i].Name).Error("failed to close fileloader")
			res = false
		}
	}
	return res
}

func GetAllFilePaths(dir string) ([]string, error) {
	var filePaths []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			filePaths = append(filePaths, absPath)
		}
		return nil
	})

	return filePaths, err
}
