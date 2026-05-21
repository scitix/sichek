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
package collector

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Parsed representation of `lldpctl -f json` output, normalized for snapshot
// consumption. The lldpctl schema uses inconsistent types (a single
// management IP is emitted as a string, but multiple as a list, etc.), so
// the on-the-wire types use json.RawMessage and helpers below smooth that
// over.

// Neighbor is the normalized view of one LLDP neighbor (per local iface).
type Neighbor struct {
	Via          string  `json:"via"`
	AgeSeconds   int64   `json:"age_seconds"`
	AgeRaw       string  `json:"age_raw,omitempty"`
	Chassis      Chassis `json:"chassis"`
	Port         Port    `json:"port"`
	VlanID       int     `json:"vlan_id,omitempty"`
	VlanPVID     bool    `json:"vlan_pvid,omitempty"`
	VlanName     string  `json:"vlan_name,omitempty"`
}

type Chassis struct {
	Name       string   `json:"name,omitempty"`
	ID         string   `json:"id,omitempty"`
	IDType     string   `json:"id_type,omitempty"`
	Descr      string   `json:"descr,omitempty"`
	MgmtIP     []string `json:"mgmt_ip,omitempty"`
	Capability []string `json:"capability,omitempty"`
}

type Port struct {
	ID             string `json:"id,omitempty"`
	IDType         string `json:"id_type,omitempty"`
	Descr          string `json:"descr,omitempty"`
	MFS            int    `json:"mfs,omitempty"`
	AggregationID  string `json:"aggregation_id,omitempty"`
	AutoNegCurrent string `json:"auto_neg_current,omitempty"`
}

// ParseLldpctlJSON converts the raw `lldpctl -f json` output into a map of
// local interface name -> Neighbor. The lldpctl output is shaped:
//
//	{"lldp":{"interface":[{"eth0":{...}},{"eth1":{...}}]}}
//
// which is awkward; we flatten it into a map. Empty input (lldpd up but no
// neighbors) yields an empty map and no error.
func ParseLldpctlJSON(raw []byte) (map[string]Neighbor, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return map[string]Neighbor{}, nil
	}

	var top struct {
		LLDP struct {
			Interface json.RawMessage `json:"interface"`
		} `json:"lldp"`
	}
	if err := json.Unmarshal([]byte(trimmed), &top); err != nil {
		return nil, fmt.Errorf("decode lldpctl top-level: %w", err)
	}
	if len(top.LLDP.Interface) == 0 || string(top.LLDP.Interface) == "null" {
		return map[string]Neighbor{}, nil
	}

	// lldpctl emits `interface` as either an array (multiple neighbors) or
	// a single object (one neighbor). Handle both.
	entries, err := splitInterfaceEntries(top.LLDP.Interface)
	if err != nil {
		return nil, err
	}

	out := make(map[string]Neighbor, len(entries))
	for _, entry := range entries {
		for ifName, rawIface := range entry {
			n, err := decodeNeighbor(rawIface)
			if err != nil {
				return nil, fmt.Errorf("decode iface %s: %w", ifName, err)
			}
			out[ifName] = n
		}
	}
	return out, nil
}

func splitInterfaceEntries(raw json.RawMessage) ([]map[string]json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, fmt.Errorf("decode interface array: %w", err)
		}
		return arr, nil
	}
	var single map[string]json.RawMessage
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, fmt.Errorf("decode interface object: %w", err)
	}
	return []map[string]json.RawMessage{single}, nil
}

// rawIface is the JSON inside the per-iface key:
//
//	{"via":..., "rid":..., "age":..., "chassis":{...}, "port":{...}, "vlan":{...}}
//
// The chassis field is yet another single-key object whose key is the
// chassis name and value is the chassis attributes.
type rawIface struct {
	Via     string          `json:"via"`
	Age     string          `json:"age"`
	Chassis json.RawMessage `json:"chassis"`
	Port    json.RawMessage `json:"port"`
	VLAN    json.RawMessage `json:"vlan"`
}

func decodeNeighbor(raw json.RawMessage) (Neighbor, error) {
	var ri rawIface
	if err := json.Unmarshal(raw, &ri); err != nil {
		return Neighbor{}, fmt.Errorf("decode iface: %w", err)
	}

	n := Neighbor{
		Via:        ri.Via,
		AgeRaw:     ri.Age,
		AgeSeconds: parseAge(ri.Age),
	}

	chassis, err := decodeChassis(ri.Chassis)
	if err != nil {
		return Neighbor{}, err
	}
	n.Chassis = chassis

	port, err := decodePort(ri.Port)
	if err != nil {
		return Neighbor{}, err
	}
	n.Port = port

	if len(ri.VLAN) > 0 {
		vlanID, pvid, vlanName := decodeVLAN(ri.VLAN)
		n.VlanID = vlanID
		n.VlanPVID = pvid
		n.VlanName = vlanName
	}
	return n, nil
}

// decodeChassis flattens the {"<name>":{...attrs...}} envelope.
func decodeChassis(raw json.RawMessage) (Chassis, error) {
	if len(raw) == 0 {
		return Chassis{}, nil
	}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return Chassis{}, fmt.Errorf("decode chassis envelope: %w", err)
	}
	for name, attrsRaw := range wrapper {
		var attrs struct {
			ID         idValue         `json:"id"`
			Descr      string          `json:"descr"`
			MgmtIP     json.RawMessage `json:"mgmt-ip"`
			Capability json.RawMessage `json:"capability"`
		}
		if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
			return Chassis{}, fmt.Errorf("decode chassis attrs: %w", err)
		}
		return Chassis{
			Name:       name,
			ID:         attrs.ID.Value,
			IDType:     attrs.ID.Type,
			Descr:      attrs.Descr,
			MgmtIP:     decodeStringList(attrs.MgmtIP),
			Capability: decodeCapabilityList(attrs.Capability),
		}, nil
	}
	return Chassis{}, nil
}

func decodePort(raw json.RawMessage) (Port, error) {
	if len(raw) == 0 {
		return Port{}, nil
	}
	var attrs struct {
		ID              idValue         `json:"id"`
		Descr           string          `json:"descr"`
		MFS             flexibleInt     `json:"mfs"`
		Aggregation     flexibleString  `json:"aggregation"`
		AutoNegotiation json.RawMessage `json:"auto-negotiation"`
	}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return Port{}, fmt.Errorf("decode port: %w", err)
	}
	p := Port{
		ID:            attrs.ID.Value,
		IDType:        attrs.ID.Type,
		Descr:         attrs.Descr,
		MFS:           int(attrs.MFS),
		AggregationID: string(attrs.Aggregation),
	}
	if len(attrs.AutoNegotiation) > 0 {
		var an struct {
			Current string `json:"current"`
		}
		_ = json.Unmarshal(attrs.AutoNegotiation, &an)
		if an.Current != "" && an.Current != "unknown" {
			p.AutoNegCurrent = an.Current
		}
	}
	return p, nil
}

func decodeVLAN(raw json.RawMessage) (vlanID int, pvid bool, vlanName string) {
	// VLAN may be a single object or an array. Take the first PVID entry if
	// possible; otherwise the first entry.
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var arr []vlanEntry
		if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
			return 0, false, ""
		}
		for _, e := range arr {
			if e.PVID {
				return int(e.VlanID), true, e.Value
			}
		}
		return int(arr[0].VlanID), arr[0].PVID, arr[0].Value
	}
	var e vlanEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		return 0, false, ""
	}
	return int(e.VlanID), e.PVID, e.Value
}

type vlanEntry struct {
	VlanID flexibleInt `json:"vlan-id"`
	PVID   bool        `json:"pvid"`
	Value  string      `json:"value"`
}

// idValue handles the {"type":"mac","value":"..."} shape used by chassis.id
// and port.id.
type idValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// flexibleInt accepts either a JSON number or a quoted-number string.
type flexibleInt int64

func (f *flexibleInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(strings.Trim(string(b), `"`))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("parse int from %q: %w", string(b), err)
	}
	*f = flexibleInt(v)
	return nil
}

// flexibleString accepts either a JSON string or a JSON number and exposes
// it as a string.
type flexibleString string

func (f *flexibleString) UnmarshalJSON(b []byte) error {
	*f = flexibleString(strings.Trim(string(b), `"`))
	return nil
}

// decodeStringList returns the values of a field that lldpctl may emit as
// either "x" or ["x","y"].
func decodeStringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var arr []string
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil
		}
		return arr
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	if s == "" {
		return nil
	}
	return []string{s}
}

// decodeCapabilityList flattens lldpctl's capability shape, which may be a
// single object or a list of objects. Returns capabilities that are
// enabled=true.
func decodeCapabilityList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	type cap struct {
		Type    string `json:"type"`
		Enabled bool   `json:"enabled"`
	}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var arr []cap
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil
		}
		out := make([]string, 0, len(arr))
		for _, c := range arr {
			if c.Enabled && c.Type != "" {
				out = append(out, c.Type)
			}
		}
		return out
	}
	var c cap
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil
	}
	if c.Enabled && c.Type != "" {
		return []string{c.Type}
	}
	return nil
}

// parseAge turns lldpctl's "55 days, 00:25:58" into a duration in seconds.
// Returns 0 if it can't be parsed; the raw string is still preserved.
func parseAge(s string) int64 {
	if s == "" {
		return 0
	}
	var days, hours, mins, secs int64
	parts := strings.Split(s, ",")
	hms := strings.TrimSpace(parts[len(parts)-1])
	if len(parts) >= 2 {
		dayPart := strings.TrimSpace(parts[0])
		dayFields := strings.Fields(dayPart)
		if len(dayFields) >= 1 {
			days, _ = strconv.ParseInt(dayFields[0], 10, 64)
		}
	}
	hmsFields := strings.Split(hms, ":")
	if len(hmsFields) == 3 {
		hours, _ = strconv.ParseInt(hmsFields[0], 10, 64)
		mins, _ = strconv.ParseInt(hmsFields[1], 10, 64)
		secs, _ = strconv.ParseInt(hmsFields[2], 10, 64)
	}
	return days*86400 + hours*3600 + mins*60 + secs
}
