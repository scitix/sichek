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
package nvidia

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"
)

type XidEventPoller struct {
	Name           string
	ErrorSupported bool
	XidEventSet    nvml.EventSet
	Ctx            context.Context
	Cancel         context.CancelFunc
	NvmlInst       nvml.Interface
	EventChan      chan *common.Result
	Cfg            *config.NvidiaUserConfig
}

func NewXidEventPoller(ctx context.Context, cfg *config.NvidiaUserConfig, nvmlInst nvml.Interface, eventChan chan *common.Result) (*XidEventPoller, error) {
	// it is ok to create and register the same/shared event set across multiple devices
	xidEventSet, err := nvml.EventSetCreate()
	if !errors.Is(err, nvml.SUCCESS) {
		logrus.WithField("component", "nvidia").Errorf("failed to create event set: %v", nvml.ErrorString(err))
		return nil, fmt.Errorf("failed to create event set: %v", nvml.ErrorString(err))
	}
	xctx, xcancel := context.WithCancel(ctx)
	return &XidEventPoller{
		Name:        "XidEvent",
		XidEventSet: xidEventSet,
		Ctx:         xctx,
		Cancel:      xcancel,
		NvmlInst:    nvmlInst,
		Cfg:         cfg,
		EventChan:   eventChan,
	}, nil
}

func (x *XidEventPoller) Start() error {
	// Get the number of devices
	numDevices, err := x.NvmlInst.DeviceGetCount()
	var retErr error
	if !errors.Is(err, nvml.SUCCESS) {
		retErr = fmt.Errorf("failed to get XidEventPoller GPU device count: %v", err)
		logrus.WithField("component", "nvidia").Warningf("%v", retErr)
		return retErr
	}

	// Get the device info
	for i := 0; i < numDevices; i++ {
		device, err := x.NvmlInst.DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			retErr = fmt.Errorf("failed to get XidEventPoller GPU device %d: %v", i, err)
			logrus.WithField("component", "nvidia").Warningf("%v", retErr)
			return retErr
		}

		supportedEvents, err := device.GetSupportedEventTypes()
		if errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
			x.ErrorSupported = false
			retErr = fmt.Errorf("GPU device %d does not support Xid events: %v", i, nvml.ErrorString(err))
			logrus.WithField("component", "nvidia").Warningf("%v", retErr)
			return retErr
		}
		if !errors.Is(err, nvml.SUCCESS) {
			retErr = fmt.Errorf("failed to get supported event types for XidEventPoller GPU device %d: %v", i, nvml.ErrorString(err))
			logrus.WithField("component", "nvidia").Warningf("%v", retErr)
			return retErr
		}
		err = device.RegisterEvents(supportedEvents, x.XidEventSet)
		if !errors.Is(err, nvml.SUCCESS) {
			retErr = fmt.Errorf("failed to register events to GPU device %d: %v", i, nvml.ErrorString(err))
			logrus.WithField("component", "nvidia").Warningf("%v", retErr)
			return retErr
		}
	}

	for {
		select {
		case <-x.Ctx.Done():
			return nil
		default:
		}

		// waits for the duration specified in x.Cfg.UpdateInterval (in seconds)
		// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlEvents.html#group__nvmlEvents
		// e, err := x.XidEventSet.Wait(uint32(x.Cfg.UpdateInterval.Microseconds()))
		e, err := x.XidEventSet.Wait(uint32(1000))

		if errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
			logrus.WithField("component", "nvidia").Warningf("XidEvent not supported -- Skipping: %v", nvml.ErrorString(err))
			continue
		}

		if errors.Is(err, nvml.ERROR_TIMEOUT) {
			// logrus.WithField("component", "nvidia").Warningf("XidEvent Not found during waiting time -- Retrying...\n")
			continue
		}

		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "nvidia").Warningf("XidEventSet.Wait failure -- Retrying: %v", nvml.ErrorString(err))
			continue
		}

		xid := e.EventData

		if !config.IsCriticalXidEvent(xid) {
			if xid != 0 {
				logrus.WithField("component", "nvidia").Warningf("received a xid event %d which is not a critical XidEvent -- skipping\n", xid)
			}
			continue
		}

		var deviceID int
		deviceID, err = e.Device.GetModuleId()
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "nvidia").Warningf("failed to get deviceID: %v\n", nvml.ErrorString(err))
			deviceID = -1
		}

		event := config.CriticalXidEvent[xid]
		event.Detail = fmt.Sprintf("GPU device %d detect critical xid event %d", deviceID, xid)
		event.Status = consts.StatusAbnormal
		logrus.WithField("component", "nvidia").Errorf("%v\n", event.Detail)

		resResult := &common.Result{
			Item:     consts.ComponentNameNvidia,
			Status:   event.Status,
			Checkers: []*common.CheckerResult{&event},
			Time:     time.Now(),
		}

		select {
		case <-x.Ctx.Done():
			return nil
		case x.EventChan <- resResult:
			logrus.WithField("component", "nvidia").Infof("Notified xid event %d for GPU device %d\n", xid, deviceID)
		default:
			logrus.WithField("component", "nvidia").Warningf("xid event channel is full, skipping event")
		}
	}
}

func (x *XidEventPoller) Stop() error {
	x.Cancel()
	if x.XidEventSet != nil {
		err := x.XidEventSet.Free()
		if !errors.Is(err, nvml.SUCCESS) {
			return fmt.Errorf("failed to free event set: %v", nvml.ErrorString(err))
		}
	}
	x.XidEventSet = nil
	return nil
}
