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
	"sync"
	"time"

	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type FileLoaderScheduler struct {
	mu        sync.Mutex
	loaders   map[string]*FileLoader
	ticker    *time.Ticker
	stopChan  chan struct{}
	interval  time.Duration
	onceStart sync.Once
}

var GlobalScheduler = &FileLoaderScheduler{
	loaders:  make(map[string]*FileLoader),
	interval: consts.DefaultFileLoaderInterval, // default interval
}

func (s *FileLoaderScheduler) Start() {
	s.loadAll()
	s.onceStart.Do(func() {
		s.stopChan = make(chan struct{})
		s.ticker = time.NewTicker(s.interval)
		go func() {
			logrus.Info("FileLoaderScheduler started")
			for {
				select {
				case <-s.ticker.C:
					s.loadAll()
				case <-s.stopChan:
					s.ticker.Stop()
					logrus.Info("FileLoaderScheduler stopped")
					return
				}
			}
		}()
	})
}

func (s *FileLoaderScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopChan != nil {
		close(s.stopChan)
		s.stopChan = nil
	}
	s.onceStart = sync.Once{} // reset, allowing restart
}

// Register adds a new FileLoader to the scheduler
func (s *FileLoaderScheduler) Register(fl *FileLoader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.loaders[fl.Name]; !exists {
		s.loaders[fl.Name] = fl
		logrus.Infof("FileLoader %s registered", fl.Name)
	}
}

// Unregister removes a FileLoader from the scheduler
func (s *FileLoaderScheduler) Unregister(fl *FileLoader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.loaders[fl.Name]; exists {
		delete(s.loaders, fl.Name)
		logrus.Infof("FileLoader %s unregistered", fl.Name)
	}
}

// loadAll calls Load on all registered FileLoaders
func (s *FileLoaderScheduler) loadAll() {
	s.mu.Lock()
	loaders := make([]*FileLoader, 0, len(s.loaders))
	for _, fl := range s.loaders {
		loaders = append(loaders, fl)
	}
	s.mu.Unlock()

	for _, fl := range loaders {
		if _, err := fl.Load(); err != nil {
			logrus.WithField("FileLoaderScheduler", fl.Name).WithError(err).Warn("Load failed in scheduler")
		}
	}
}

// SetInterval enables changing the interval at which the scheduler runs
// It stops the current ticker and starts a new one with the new interval.
func (s *FileLoaderScheduler) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d <= 0 {
		return
	}
	s.interval = d
	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = time.NewTicker(s.interval)
	}
}
