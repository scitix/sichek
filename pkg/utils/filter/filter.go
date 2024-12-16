package filter

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type FilterResult struct {
	Name     string
	Regex    string
	FileName string
	Line     string
}

type Filter struct {
	Regex         []*RegexFilter
	CommandFilter *CommandFilter
	FileFilter    *FileFilter
}

func NewFilter(regexpName []string, regexps []string, filesName []string, cmds [][]string, cacheLineN int64) (*Filter, error) {
	return NewFilterSkip(regexpName, regexps, filesName, cmds, cacheLineN, 100)
}

func NewFilterSkip(regexpName []string, regexps []string, filesName []string, cmds [][]string, cacheLineN int64, skip_percent int64) (*Filter, error) {
	if len(regexpName) != len(regexps) {
		logrus.Error("wrong input, u need spesify a name for each regexps")
		return nil, fmt.Errorf("No Name specified for regexp")
	}

	var res Filter

	for i := 0; i < len(regexps); i++ {
		res.Regex = append(res.Regex, NewRegexFilter(regexpName[i], regexps[i]))
	}
	var err error
	res.FileFilter, err = NewFileFilterWithRegSkip(res.Regex, filesName, cacheLineN, skip_percent)
	if err != nil {
		logrus.WithError(err).Error("failed to create file filter in filter new")
		return nil, err
	}
	res.CommandFilter, err = NewCommandFilterWithReg(res.Regex, cmds, cacheLineN)
	if err != nil {
		logrus.WithError(err).Error("failed to create command filter in filter new")
		return nil, err
	}
	return &res, nil
}

func (f *Filter) Check() []FilterResult {
	var res []FilterResult
	res = append(res, f.FileFilter.Check()...)
	res = append(res, f.CommandFilter.Check()...)
	return res
}

func (f *Filter) Close() bool {
	return f.FileFilter.Close() && f.CommandFilter.Close()
}
