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
package dmesg

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type KmsgReader struct {
	// file uses io.Reader interface instead of *os.File to enable dependency injection
	// for unit testing while maintaining production functionality. *os.File implements io.Reader.
	file        io.Reader
	skipPercent int64
	stop        chan struct{}
}

func NewKmsgReader(r io.Reader, skipPercent int64) (*KmsgReader, error) {
	// f, err := os.Open("/dev/kmsg")
	// if err != nil {
	//     return nil, err
	// }
	return &KmsgReader{
		file:        r,
		skipPercent: skipPercent,
		stop:        make(chan struct{}),
	}, nil
}

func (r *KmsgReader) Start(onLine func(string)) {
	if r.skipPercent == 100 {
		if seeker, ok := r.file.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekEnd); err != nil {
				logrus.WithError(err).Error("seek /dev/kmsg to tail failed")
			} else {
				logrus.Info("kmsg reader starts from tail (skipPercent=100)")
			}
		} else {
			scanner := bufio.NewScanner(r.file)
			for scanner.Scan() {
				// skip all lines before
			}
			logrus.Warn("skipPercent=100 in Unit Test")
		}
	} else {
		logrus.Info("kmsg reader starts from head (skipPercent=0)")
	}

	// Base time：wall clock + monotonic
	wallNow := time.Now()
	var ts unix.Timespec
	_ = unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	monoNow := time.Duration(ts.Sec)*time.Second + time.Duration(ts.Nsec)

	go func() {
		scanner := bufio.NewScanner(r.file)
		for scanner.Scan() {
			select {
			case <-r.stop:
				return
			default:
			}

			line := scanner.Text()
			// <pri>,<seq>,<ts>,<flags>;message
			idx := strings.Index(line, ";")
			if idx == -1 {
				continue
			}

			meta := line[:idx]
			msg := line[idx+1:]

			fields := strings.Split(meta, ",")
			if len(fields) < 3 {
				continue
			}

			tsNano, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil {
				continue
			}

			// monotonic → wall clock
			eventMono := time.Duration(tsNano)
			eventTime := wallNow.Add(eventMono - monoNow)

			// dmesg -T Style
			prefix := eventTime.Format("[Mon Jan _2 15:04:05 2006] ")

			onLine(prefix + msg)
		}

		if err := scanner.Err(); err != nil {
			logrus.WithError(err).Error("read /dev/kmsg failed")
		}
	}()
}

func (r *KmsgReader) Stop() {
	close(r.stop)
	// if r.file is *os.File, it can be closed
	if closer, ok := r.file.(io.Closer); ok {
		closer.Close()
	}
}
