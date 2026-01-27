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
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type XidEventPoller struct {
	Name           string
	ErrorSupported bool
	NvmlInst       nvml.Interface
	NvmlMtx        *sync.RWMutex

	Cfg       *config.NvidiaUserConfig
	EventChan chan *common.Result

	Ctx    context.Context
	Cancel context.CancelFunc

	XidEventSet nvml.EventSet
	wg          sync.WaitGroup // Wait for Start() to exit
}

func NewXidEventPoller(ctx context.Context, cfg *config.NvidiaUserConfig, nvmlInst nvml.Interface, nvmlMtx *sync.RWMutex, eventChan chan *common.Result) (*XidEventPoller, error) {
	xidEventSet, ret := nvmlInst.EventSetCreate()
	if ret != nvml.SUCCESS {
		logrus.WithField("component", "nvidia").Errorf("failed to create event set: %v", nvml.ErrorString(ret))
		return nil, fmt.Errorf("failed to create event set: %v", nvml.ErrorString(ret))
	}
	xctx, xcancel := context.WithCancel(ctx)
	return &XidEventPoller{
		Name:           "XidEventPoller",
		ErrorSupported: true,
		NvmlInst:       nvmlInst,
		NvmlMtx:        nvmlMtx,
		Cfg:            cfg,
		EventChan:      eventChan,
		Ctx:            xctx,
		Cancel:         xcancel,
		XidEventSet:    xidEventSet,
	}, nil
}

func (x *XidEventPoller) Start() error {
	x.wg.Add(1)
	defer x.wg.Done()

	if err := x.registerDevices(); err != nil {
		return err
	}
	logrus.WithField("component", "nvidia").Infof(" %s Started", x.Name)

	for {
		select {
		case <-x.Ctx.Done():
			logrus.WithField("component", "nvidia").Infof(" %s stopped", x.Name)
			return nil
		default:
		}

		// waits for the duration specified in x.Cfg.UpdateInterval (in seconds)
		// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlEvents.html#group__nvmlEvents
		// e, err := x.XidEventSet.Wait(uint32(x.Cfg.UpdateInterval.Microseconds()))
		event, ret := x.XidEventSet.Wait(200)
		logrus.WithField("component", "nvidia").Infof("XidEventSet.Wait returned: %v, %v", event.EventData, ret)
		if ret == nvml.ERROR_NOT_SUPPORTED {
			logrus.WithField("component", "nvidia").Warningf("XidEvent not supported -- Skipping: %v", nvml.ErrorString(ret))
			continue
		}
		if ret == nvml.ERROR_TIMEOUT {
			// no event within timeout
			continue
		}
		if ret != nvml.SUCCESS {
			logrus.WithField("component", "nvidia").Warningf("XidEventSet.Wait failure -- Retrying: %v", nvml.ErrorString(ret))
			continue
		}

		x.handleEvent(event)
	}
}

func (x *XidEventPoller) registerDevices() error {
	x.NvmlMtx.RLock()
	defer x.NvmlMtx.RUnlock()

	numDevices, ret := x.NvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		retErr := fmt.Errorf("failed to get XidEventPoller GPU device count: %v", nvml.ErrorString(ret))
		logrus.WithField("component", "nvidia").Warningf("%v", retErr)
		return retErr
	}

	// Only register XID critical error events
	xidEventType := uint64(nvml.EventTypeXidCriticalError)
	registeredCount := 0

	for i := 0; i < numDevices; i++ {
		device, ret := x.NvmlInst.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			logrus.WithField("component", "nvidia").Warningf("failed to get XidEventPoller GPU device %d: %v, skipping", i, nvml.ErrorString(ret))
			continue
		}

		// Check if device supports XID events
		supportedEvents, ret := device.GetSupportedEventTypes()
		if ret == nvml.ERROR_NOT_SUPPORTED {
			logrus.WithField("component", "nvidia").Warningf("GPU device %d does not support events: %v, skipping", i, nvml.ErrorString(ret))
			continue
		} else if ret != nvml.SUCCESS {
			logrus.WithField("component", "nvidia").Warningf("failed to get supported event types for GPU device %d: %v, skipping", i, nvml.ErrorString(ret))
			continue
		}

		// Check if XID events are supported
		if supportedEvents&xidEventType == 0 {
			logrus.WithField("component", "nvidia").Warningf("GPU device %d does not support XID critical error events (supported: 0x%x), skipping", i, supportedEvents)
			continue
		}

		// Register only XID critical error events
		ret = device.RegisterEvents(xidEventType, x.XidEventSet)
		if ret != nvml.SUCCESS {
			logrus.WithField("component", "nvidia").Warningf("failed to register XID events to GPU device %d: %v, skipping", i, nvml.ErrorString(ret))
			continue
		}

		registeredCount++
		logrus.WithField("component", "nvidia").Infof("Successfully registered XID events for GPU device %d", i)
	}

	if registeredCount == 0 {
		x.ErrorSupported = false
		return fmt.Errorf("no GPU devices support XID events or failed to register XID events")
	}

	logrus.WithField("component", "nvidia").Infof("Successfully registered XID events for %d GPU device(s)", registeredCount)
	return nil
}

func (x *XidEventPoller) handleEvent(e nvml.EventData) {
	// Verify this is an XID critical error event
	if e.EventType != nvml.EventTypeXidCriticalError {
		logrus.WithField("component", "nvidia").Debugf("received non-XID event (type: 0x%x), skipping", e.EventType)
		return
	}

	xid := e.EventData
	if !config.IsCriticalXidEvent(xid) {
		if xid != 0 {
			logrus.WithField("component", "nvidia").Warningf("received a xid event %d which is not a critical XidEvent -- skipping\n", xid)
		}
		return
	}

	x.NvmlMtx.RLock()
	deviceID, ret := e.Device.GetModuleId()
	x.NvmlMtx.RUnlock()
	if ret != nvml.SUCCESS {
		logrus.WithField("component", "nvidia").Warningf("failed to get deviceID: %v", nvml.ErrorString(ret))
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
	case x.EventChan <- resResult:
		logrus.WithField("component", "nvidia").Infof("Notified xid event %d for GPU device %d", xid, deviceID)
	default:
		logrus.WithField("component", "nvidia").Warningf("xid event channel is full, skipping event")
	}
}

func (x *XidEventPoller) Stop() error {
	// Cancel context to signal Start() to exit
	x.Cancel()

	// Wait for Start() to actually exit (bounded wait)
	done := make(chan struct{})
	go func() {
		x.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		logrus.WithField("component", "nvidia").Warningf("timed out waiting for XidEventPoller Start() to exit")
	}

	if x.XidEventSet != nil {
		ret := x.XidEventSet.Free()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("failed to free event set: %v", nvml.ErrorString(ret))
		}
		x.XidEventSet = nil
	}

	return nil
}
