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
package ethernet

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/ethernet/checker"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
)

func TestEthernetCheckers(t *testing.T) {
	spec := &config.EthernetSpecConfig{
		BondMode:       "802.3ad",
		MIIStatus:      "up",
		LACPRate:       "fast",
		MTU:            "1500",
		MinSlaves:      1,
		XmitHashPolicy: "layer3+4",
		Miimon:         100,
		UpDelay:        100,
		DownDelay:      100,
	}

	info := &collector.EthernetInfo{
		BondInterfaces: []string{"bond0"},
		Bonds: map[string]collector.BondState{
			"bond0": {
				Name:           "bond0",
				IsUp:           true,
				HasLowerUp:     true,
				IPAddr:         "192.168.1.1",
				MTU:            1500,
				Mode:           "802.3ad",
				Miimon:         100,
				XmitHashPolicy: "layer3+4",
				LACPRate:       "fast",
				ActiveSlave:    "eth0",
			},
		},
		LACP: map[string]collector.LACPState{
			"bond0": {
				ActiveAggregatorID: "1",
				ActorKey:           "21",
				PartnerKey:         "40016",
				PartnerMacAddress:  "11:22:33:44:55:66",
				SlaveAggregatorIDs: map[string]string{"eth0": "1"},
				SlaveActorKeys:     map[string]string{"eth0": "21"},
				SlavePartnerKeys:   map[string]string{"eth0": "40016"},
			},
		},
		Slaves: map[string]map[string]collector.SlaveState{
			"bond0": {
				"eth0": {Name: "eth0", IsUp: true, LinkDetected: true, Speed: 25000, Duplex: "Full"},
			},
		},
		Stats: map[string]collector.TrafficStats{
			"eth0": {RXErrors: 0, TXErrors: 0, Dropped: 0, Carrier: 0},
		},
		Routes: collector.RouteState{
			DefaultRouteViaBond: true,
			GatewayReachable:    true,
			GatewayIP:           "192.168.1.1",
		},
		SyslogErrors: []string{},
		ProcNetBonding: map[string]string{
			"bond0": "MII Status: up\nUp Delay (ms): 100\nDown Delay (ms): 100\n",
		},
		RPFilter: map[string]string{
			"all":   "0",
			"bond0": "0",
		},
	}

	ctx := context.Background()

	checkers, err := checker.NewCheckers(&config.EthernetUserConfig{}, spec)
	assert.NoError(t, err)

	for _, c := range checkers {
		res, err := c.Check(ctx, info)
		assert.NoError(t, err)
		assert.Equal(t, consts.StatusNormal, res.Status, "Checker %s failed unexpectedly: %s", c.Name(), res.Detail)
	}
}

func TestEthernetCheckersFailures(t *testing.T) {
	spec := &config.EthernetSpecConfig{
		BondMode:       "802.3ad",
		MIIStatus:      "up",
		LACPRate:       "fast",
		MTU:            "1500",
		MinSlaves:      2,
		XmitHashPolicy: "layer3+4",
		Miimon:         100,
		UpDelay:        100,
		DownDelay:      100,
	}

	info := &collector.EthernetInfo{
		BondInterfaces: []string{"bond0"},
		Bonds: map[string]collector.BondState{
			"bond0": {
				Name:           "bond0",
				IsUp:           false,
				HasLowerUp:     false,
				IPAddr:         "",
				MTU:            1500,
				Mode:           "802.3ad",
				Miimon:         0,
				XmitHashPolicy: "layer3+4",
				LACPRate:       "slow",
				ActiveSlave:    "",
			},
		},
		LACP: map[string]collector.LACPState{
			"bond0": {
				ActiveAggregatorID: "1",
				ActorKey:           "21",
				PartnerKey:         "40016",
				PartnerMacAddress:  "00:00:00:00:00:00",
				SlaveAggregatorIDs: map[string]string{"eth0": "2"}, // mismatch
				SlaveActorKeys:     map[string]string{"eth0": "21"},
				SlavePartnerKeys:   map[string]string{"eth0": "40016"},
			},
		},
		Slaves: map[string]map[string]collector.SlaveState{
			"bond0": {
				"eth0": {Name: "eth0", IsUp: false, LinkDetected: false, Speed: 1000, Duplex: "Half"},
			},
		},
		Stats: map[string]collector.TrafficStats{
			"eth0": {RXErrors: 100, TXErrors: 50, Dropped: 100, Carrier: 10},
		},
		Routes: collector.RouteState{
			DefaultRouteViaBond: false,
			GatewayReachable:    false,
			GatewayIP:           "192.168.1.1",
		},
		SyslogErrors: []string{"eth0 tx timeout"},
		RPFilter: map[string]string{
			"all":   "1",
			"bond0": "1",
		},
	}

	ctx := context.Background()

	checkers, err := checker.NewCheckers(&config.EthernetUserConfig{}, spec)
	assert.NoError(t, err)

	for _, c := range checkers {
		res, err := c.Check(ctx, info)
		assert.NoError(t, err)
		assert.Equal(t, consts.StatusAbnormal, res.Status, "Checker %s should have failed", c.Name())
	}
}
