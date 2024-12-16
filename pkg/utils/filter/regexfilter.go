package filter

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
