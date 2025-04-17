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
package common

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Component interface {
	Name() string
	HealthCheck(ctx context.Context) (*Result, error)
	GetTimeout() time.Duration

	// CacheResults cached analyze results
	CacheResults() ([]*Result, error)
	LastResult() (*Result, error)
	// CacheInfos cached collector infos
	CacheInfos() ([]Info, error)
	LastInfo() (Info, error)
	PrintInfo(Info, *Result, bool) bool

	// Metrics For http service
	Metrics(ctx context.Context, since time.Time) (interface{}, error)

	// Start For daemon service
	Start() <-chan *Result
	Update(cfg ComponentUserConfig) error
	Status() bool
	Stop() error
}

type Result struct {
	Item       string           `json:"item"`
	Node       string           `json:"node"`
	Status     string           `json:"status"`
	Level      string           `json:"level"`
	RawData    string           `json:"raw_data"`
	Suggestion string           `json:"suggest"`
	Checkers   []*CheckerResult `json:"checkers"`
	Time       time.Time        `json:"time"`
}

func (r *Result) JSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	return string(data), err
}

type CheckerResult struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Device      string            `json:"device"`
	Spec        string            `json:"spec"`
	Curr        string            `json:"curr"`
	Status      string            `json:"status"`
	Level       string            `json:"level"`
	Suggestion  string            `json:"suggest"`
	Detail      string            `json:"detail"`
	ErrorName   string            `json:"error_name"`
	Labels      map[string]string `json:"labels"`
}

func (c *CheckerResult) JSON() ([]byte, error) {
	return JSON(c)
}

func (c *CheckerResult) ToString() string {
	return ToString(c)
}

type CommonService struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      ComponentUserConfig
	cfgMutex sync.RWMutex

	healthCheckFunc HealthCheckFunc
	checkTimeout    time.Duration
	componentName   string

	mutex         sync.RWMutex
	running       bool
	resultChannel chan *Result
}

type HealthCheckFunc func(ctx context.Context) (*Result, error)

func NewCommonService(ctx context.Context, cfg ComponentUserConfig, componentName string, checkTimeout time.Duration, analyze HealthCheckFunc) *CommonService {
	cctx, ccancel := context.WithCancel(ctx)

	return &CommonService{
		ctx:             cctx,
		cancel:          ccancel,
		cfg:             cfg,
		checkTimeout:    checkTimeout,
		healthCheckFunc: analyze,
		componentName:   componentName,
		resultChannel:   make(chan *Result),
	}
}

func (s *CommonService) Start() <-chan *Result {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return s.resultChannel
	}
	s.running = true
	s.mutex.Unlock()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("component", "service").Errorf("recover panic err: %v\n", err)
			}
		}()
		interval := s.cfg.GetQueryInterval()
		ticker := time.NewTicker(interval * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				// Check if need to update ticker
				newInterval := s.cfg.GetQueryInterval()
				if newInterval != interval {
					logrus.WithField("component", "NVIDIA").Infof("Updating ticker interval from %v to %v", interval*time.Second, newInterval*time.Second)
					ticker.Stop()
					ticker = time.NewTicker(newInterval * time.Second)
					interval = newInterval
				}
				s.mutex.Lock()
				result, err := RunHealthCheckWithTimeout(s.ctx, s.checkTimeout, s.componentName, s.healthCheckFunc)
				s.mutex.Unlock()
				if err != nil {
					logrus.WithField("component", s.componentName).Errorf("Run HealthCheck func error: %v", err)
					continue
				}

				s.mutex.Lock()
				s.resultChannel <- result
				s.mutex.Unlock()
			}
		}
	}()
	s.mutex.Lock()
	s.running = true
	s.mutex.Unlock()
	return s.resultChannel
}

// Stop 用于systemD的停止
func (s *CommonService) Stop() error {
	s.cancel()
	s.mutex.Lock()
	close(s.resultChannel)
	s.running = false
	s.mutex.Unlock()
	return nil

}

// Update 更新组件的配置信息，比如采样周期
func (s *CommonService) Update(cfg ComponentUserConfig) error {
	s.cfgMutex.Lock()
	s.cfg = cfg
	s.cfgMutex.Unlock()
	return nil
}

// Status 返回组件的运行情况
func (s *CommonService) Status() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.running
}
