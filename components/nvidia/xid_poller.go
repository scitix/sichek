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
	"fmt"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"

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
	Cfg            config.ComponentConfig
}

func NewXidEventPoller(ctx context.Context, cfg config.ComponentConfig, nvmlInst nvml.Interface, eventChan chan *common.Result) (*XidEventPoller, error) {
	// it is ok to create and register the same/shared event set across multiple devices
	xidEventSet, err := nvml.EventSetCreate()
	if err != nvml.SUCCESS {
		logrus.WithField("component", "Nvidia").Errorf("failed to create event set: %v", nvml.ErrorString(err))
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
	if err != nvml.SUCCESS {
		retErr = fmt.Errorf("failed to get XidEventPoller GPU device count: %v", err)
		logrus.WithField("component", "Nvidia").Warningf("%v", retErr)
		return retErr
	}

	// Get the device info
	for i := 0; i < numDevices; i++ {
		device, err := x.NvmlInst.DeviceGetHandleByIndex(i)
		if err != nvml.SUCCESS {
			retErr = fmt.Errorf("failed to get XidEventPoller GPU device %d: %v", i, err)
			logrus.WithField("component", "Nvidia").Warningf("%v", retErr)
			return retErr
		}

		supportedEvents, err := device.GetSupportedEventTypes()
		if err == nvml.ERROR_NOT_SUPPORTED {
			x.ErrorSupported = false
			retErr = fmt.Errorf("GPU device %d does not support Xid events: %v", i, nvml.ErrorString(err))
			logrus.WithField("component", "Nvidia").Warningf("%v", retErr)
			return retErr
		}
		if err != nvml.SUCCESS {
			retErr = fmt.Errorf("failed to get supported event types for XidEventPoller GPU device %d: %v", i, nvml.ErrorString(err))
			logrus.WithField("component", "Nvidia").Warningf("%v", retErr)
			return retErr
		}
		err = device.RegisterEvents(supportedEvents, x.XidEventSet)
		if err != nvml.SUCCESS {
			retErr = fmt.Errorf("failed to register events to GPU device %d: %v", i, nvml.ErrorString(err))
			logrus.WithField("component", "Nvidia").Warningf("%v", retErr)
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

		if err == nvml.ERROR_NOT_SUPPORTED {
			logrus.WithField("component", "Nvidia").Warningf("XidEvent not supported -- Skipping: %v", nvml.ErrorString(err))
			continue
		}

		if err == nvml.ERROR_TIMEOUT {
			logrus.WithField("component", "Nvidia").Warningf("XidEvent Not found during waiting time -- Retrying...\n")
			continue
		}

		if err != nvml.SUCCESS {
			logrus.WithField("component", "Nvidia").Warningf("XidEventSet.Wait failure -- Retrying: %v", nvml.ErrorString(err))
			continue
		}

		xid := e.EventData

		if !config.IsCriticalXidEvent(xid) {
			if xid != 0 {
				logrus.WithField("component", "Nvidia").Warningf("received a xid event %d which is not a critical XidEvent -- skipping\n", xid)
			}
			continue
		}

		var deviceID int
		deviceID, err = e.Device.GetModuleId()
		if err != nvml.SUCCESS {
			logrus.WithField("component", "Nvidia").Warningf("failed to get deviceID: %v\n", nvml.ErrorString(err))
			deviceID = -1
		}

		event := config.CriticalXidEvent[xid]
		event.Detail = fmt.Sprintf("GPU device %d detect critical xid event %d", deviceID, xid)
		event.Status = "abnormal"
		logrus.WithField("component", "Nvidia").Errorf("%v\n", event.Detail)

		res_result := &common.Result{
			Item:     x.Name,
			Status:   event.Status,
			Checkers: []*common.CheckerResult{&event},
			Time:     time.Now(),
		}

		select {
		case <-x.Ctx.Done():
			return nil
		case x.EventChan <- res_result:
			logrus.WithField("component", "Nvidia").Infof("Notified xid event %d for GPU device %d\n", xid, deviceID)
		default:
			logrus.WithField("component", "Nvidia").Warningf("xid event channel is full, skipping event")
		}
	}
}

func (x *XidEventPoller) Stop() error {
	x.Cancel()
	if x.XidEventSet != nil {
		err := x.XidEventSet.Free()
		if err != nvml.SUCCESS {
			return fmt.Errorf("failed to free event set: %v", nvml.ErrorString(err))
		}
	}
	x.XidEventSet = nil
	return nil
}
