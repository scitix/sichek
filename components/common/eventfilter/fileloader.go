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
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

var Name2FileLoader sync.Map

type FileLoader struct {
	Name        string
	FD          *os.File
	Inode       uint64   // inode，used to detect if the file has rotated
	CachedLines []string // ring buffer with fixed cache size
	CacheNum    int64
	LogLineNum  int64         // total log line number, same as the next Pos to read
	Pos         int64         // file read offset, used to read new lines
	reader      *bufio.Reader // buffered reader for reading lines
	rw          sync.RWMutex  // for high concurrency read access
}

func NewFileLoader(fileName string, cacheNum int64, skipPercent int64) (*FileLoader, error) {
	value, exists := Name2FileLoader.Load(fileName)
	if exists {
		logrus.WithField("FileLoader", fileName).Warn("file loader already exists, return existing")
		if fl, ok := value.(*FileLoader); ok {
			return fl, nil
		} else {
			logrus.WithField("FileLoader", fileName).Warn("existing file loader in map has wrong type, deleting and recreating")
			Name2FileLoader.Delete(fileName)
		}
	}
	fl := &FileLoader{
		Name:        fileName,
		FD:          nil,
		CachedLines: make([]string, cacheNum),
		CacheNum:    cacheNum,
		LogLineNum:  0,
		Pos:         0,
	}
	if err := fl.Open(); err != nil {
		logrus.WithField("FileLoader", fileName).WithError(err).Warn("failed to open file in file loader")
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	// set the inode of the file, used to detect if the file has rotated
	if err := fl.updateInode(); err != nil {
		fl.Close()
		return nil, fmt.Errorf("failed to get inode: %w", err)
	}
	if skipPercent >= 0 && skipPercent <= 100 {
		fileSize, err := fl.GetFileSize()
		if err != nil {
			fl.Close()
			return nil, fmt.Errorf("failed to get file size: %w", err)
		}
		fl.Pos = fileSize * skipPercent / 100
	} else {
		logrus.WithField("FileLoader", fileName).Warnf("failed to skip %d file content in file loader", skipPercent)
	}

	Name2FileLoader.Store(fileName, fl)
	GlobalScheduler.Register(fl)
	GlobalScheduler.Start()
	return fl, nil
}

func (f *FileLoader) Open() error {
	f.rw.Lock()
	defer f.rw.Unlock()
	if f.FD != nil {
		return nil
	}
	var err error
	f.FD, err = os.OpenFile(f.Name, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to open file")
		return err
	}
	return nil
}

func (f *FileLoader) Close() error {
	f.rw.Lock()
	defer f.rw.Unlock()
	if f.FD == nil {
		logrus.WithField("FileLoader", f.Name).Debug("file already closed")
		return nil
	}

	err := f.FD.Close()
	f.FD = nil
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to close file")
		return err
	}

	Name2FileLoader.Delete(f.Name)
	GlobalScheduler.Unregister(f)
	return nil
}

func (f *FileLoader) GetFileSize() (int64, error) {
	f.rw.Lock()
	defer f.rw.Unlock()
	var res int64 = 0
	stat, err := f.FD.Stat()
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to get file stat")
		return res, err
	}

	res = stat.Size()
	return res, nil
}

// updateInode get the inode of current opened file, to detect rotate
func (f *FileLoader) updateInode() error {
	if f.FD == nil {
		return errors.New("file not opened")
	}
	stat, err := f.FD.Stat()
	if err != nil {
		return err
	}
	sys := stat.Sys()
	if sys == nil {
		return errors.New("file stat sys is nil")
	}
	stat_t, ok := sys.(*syscall.Stat_t)
	if !ok {
		return errors.New("not syscall.Stat_t type")
	}
	f.Inode = stat_t.Ino
	return nil
}

// reopenFile reopens the file and updates the inode.
func (f *FileLoader) reopenFile() error {
	if err := f.Close(); err != nil {
		return err
	}
	if err := f.Open(); err != nil {
		return err
	}
	if err := f.updateInode(); err != nil {
		return err
	}
	f.Pos = 0
	f.LogLineNum = 0
	return nil
}

// Load reads new lines from the file and caches them to ring buffer. max lines to load one time is CacheNum.
func (f *FileLoader) Load() (int64, error) {
	f.rw.Lock()
	defer f.rw.Unlock()
	stat, err := f.FD.Stat()
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithError(err).Error("failed to get file stat")
		return 0, err
	}

	// reopen the file when it retates
	sys := stat.Sys()
	stat_t, ok := sys.(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("file stat sys cast error")
	}
	if stat_t.Ino != f.Inode {
		logrus.WithField("FileLoader", f.Name).Info("file rotated, reopen")
		if err := f.reopenFile(); err != nil {
			return 0, fmt.Errorf("failed to reopen rotated file: %w", err)
		}
		stat = nil
		stat, err = f.FD.Stat()
		if err != nil {
			return 0, err
		}
	}

	fileSize := stat.Size()
	if f.Pos >= fileSize {
		return 0, nil
	}

	_, err = f.FD.Seek(f.Pos, 0)
	if err != nil {
		logrus.WithField("FileLoader", f.Name).WithField("Pos", f.Pos).WithError(err).Error("failed to seek file")
		return 0, err
	}
	f.reader = bufio.NewReader(f.FD)
	var linesRead int64 = 0
	for linesRead < f.CacheNum {
		line, err := f.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logrus.WithField("FileLoader", f.Name).WithField("Pos", f.Pos).WithError(err).Error("failed to read file")
			return linesRead, err
		}
		// write the line to the cache
		idx := f.LogLineNum % f.CacheNum
		f.CachedLines[idx] = line
		f.LogLineNum++
		linesRead++
		f.Pos += int64(len(line))
		if err == io.EOF {
			break
		}
	}
	return linesRead, nil
}
