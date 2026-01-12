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
	"strings"

	"github.com/sirupsen/logrus"
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
			// /dev/kmsg, *os.File
			if _, err := seeker.Seek(0, io.SeekEnd); err != nil {
				logrus.WithError(err).Error("seek /dev/kmsg to tail failed")
			} else {
				logrus.Info("kmsg reader starts from tail (skipPercent=100)")
			}
		} else {
			scanner := bufio.NewScanner(r.file)
			for scanner.Scan() {
				// skip all lines
			}
			logrus.Warn("skipPercent=100 in Unit Test)")
		}
	} else {
		logrus.Info("kmsg reader starts from head (skipPercent=0)")
	}

	go func() {
		scanner := bufio.NewScanner(r.file)
		for scanner.Scan() {
			select {
			case <-r.stop:
				return
			default:
			}

			line := scanner.Text()
			// /dev/kmsg format: <pri>,<seq>,<ts>,<flags>;message
			if idx := strings.Index(line, ";"); idx != -1 {
				onLine(line[idx+1:])
			}
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
