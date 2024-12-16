package filter

import (
	"bufio"
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var Name2FileLoader sync.Map

type FileLoader struct {
	Name string
	Pos  int64

	FD *os.File

	CachedLines []string
	CacheNum    int64
	LogLineNum  int64
}

func NewFileLoader(fileName string, cacheNum int64, skip_percent int64) *FileLoader {
	value, exists := Name2FileLoader.Load(fileName)
	if exists {
		logrus.WithField("FileLoader", fileName).Warn("failed to new file loader, because it has existed")
		if res, ok := value.(*FileLoader); ok {
			return res
		} else {
			logrus.WithField("FileLoader", fileName).Warn("prev value in file loader map type isnot *FileLoader")
		}
	}
	res := &FileLoader{
		Name:        fileName,
		Pos:         0,
		FD:          nil,
		CachedLines: make([]string, cacheNum),
		CacheNum:    cacheNum,
		LogLineNum:  0,
	}
	if !res.Open() {
		logrus.WithField("FileLoader", fileName).Warn("failed to open file in file loader")
		return nil
	}
	if skip_percent >= 0 && skip_percent <= 100 {
		file_size, _ := res.GetFileSize()
		res.Pos = file_size * skip_percent / 100
	} else {
		logrus.WithField("FileLoader", fileName).Warnf("failed to skip %d file content in file loader", skip_percent)
	}

	Name2FileLoader.Store(fileName, res)
	return res
}

func (f *FileLoader) Open() bool {
	var err error
	f.FD, err = os.OpenFile(f.Name, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to open file")
		return false
	}
	return true
}

func (f *FileLoader) Close() bool {
	if f.FD == nil {
		logrus.WithField("FileLoader", f.Name).Error("no open FD in FD Close")
		return true
	}

	err := f.FD.Close()
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to close file")
		return false
	}

	Name2FileLoader.Delete(f.Name)
	return true
}

func (f *FileLoader) HasUpdate() bool {
	if f.FD == nil {
		logrus.WithField("FileLoader", f.Name).Error("no open FD in Update check")
		return false
	}

	stat, err := f.FD.Stat()
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to get file stat")
		return false
	}

	if stat.Size() > f.Pos {
		return true
	}
	return false
}

func (f *FileLoader) Load() (int64, error) {
	var res int64 = 0

	for {
		lines, err := f.GetLines(f.Pos)
		if err != nil {
			logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to get new line")
			return 0, err
		}
		if len(lines) == 0 {
			break
		}

		for i := 0; i < len(lines); i++ {
			f.CachedLines[f.LogLineNum%f.CacheNum] = lines[i]
			f.Pos += int64(len(lines[i]))
			f.LogLineNum++
			res++
		}
	}

	return res, nil
}

func (f *FileLoader) GetFileSize() (int64, error) {
	var res int64 = 0
	stat, err := f.FD.Stat()
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to get file stat")
		return res, err
	}

	res = stat.Size()
	return res, nil
}

func (f *FileLoader) GetLines(beginPos int64) ([]string, error) {
	var res []string

	stat, err := f.FD.Stat()
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to get file stat")
		return res, err
	}

	file_size := stat.Size()
	if beginPos > file_size {
		return res, nil
	}

	_, err = f.FD.Seek(beginPos, 0)
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to seek file to %ld", beginPos)
		return res, err
	}
	loader := bufio.NewReader(f.FD)

	oneLoadNum := int(f.CacheNum)
	if oneLoadNum < 1000000 {
		oneLoadNum = 1000000
	}
	for len(res) < oneLoadNum && beginPos < file_size {
		line, err := loader.ReadString('\n')
		if err != nil && err != io.EOF {
			logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to read file at %ld", beginPos)
			return res, err
		}
		beginPos += int64(len(line))
		res = append(res, line)
	}
	return res, nil
}
