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
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// PTPInfo holds PTP and NTP clock synchronization status.
type PTPInfo struct {
	PTPServiceActive bool    `json:"ptp_service_active"`
	PHC2SysActive    bool    `json:"phc2sys_active"`
	OffsetNs         float64 `json:"offset_ns"`
	NTPServiceActive bool    `json:"ntp_service_active"`
	NTPOffset        float64 `json:"ntp_offset_ns"`
	SyncAvailable    bool    `json:"sync_available"`
}

// Get populates PTPInfo by checking ptp4l, phc2sys, chrony, and ntpd services.
func (p *PTPInfo) Get() {
	p.PTPServiceActive = isServiceActive("ptp4l")
	p.PHC2SysActive = isServiceActive("phc2sys")

	if p.PTPServiceActive {
		offset, err := getPTP4LOffset()
		if err == nil {
			p.OffsetNs = offset
			p.SyncAvailable = true
			return
		}
	}

	// Fallback to NTP (chrony or ntpd)
	if isServiceActive("chronyd") || isServiceActive("chrony") {
		p.NTPServiceActive = true
		offset, err := getChronycOffset()
		if err == nil {
			p.NTPOffset = offset
			p.SyncAvailable = true
			return
		}
	}

	if isServiceActive("ntpd") || isServiceActive("ntp") {
		p.NTPServiceActive = true
		p.SyncAvailable = true
	}
}

// OffsetMs returns the absolute offset in milliseconds.
// It uses PTP offset if available, otherwise NTP offset.
func (p *PTPInfo) OffsetMs() float64 {
	if p.PTPServiceActive {
		return math.Abs(p.OffsetNs) / 1e6
	}
	return math.Abs(p.NTPOffset) / 1e6
}

// isServiceActive checks if a systemd service is active.
func isServiceActive(service string) bool {
	err := exec.Command("systemctl", "is-active", "--quiet", service).Run()
	return err == nil
}

// getPTP4LOffset retrieves the latest PTP master offset from journalctl.
func getPTP4LOffset() (float64, error) {
	out, err := exec.Command("journalctl", "-u", "ptp4l", "-n", "10", "--no-pager", "-q").Output()
	if err != nil {
		return 0, err
	}
	return parsePTP4LOffset(string(out))
}

// parsePTP4LOffset extracts "master offset" value from ptp4l log lines.
// Example line: "ptp4l[1234.567]: master offset        -23 s2 freq   +1234 path delay       456"
func parsePTP4LOffset(logOutput string) (float64, error) {
	re := regexp.MustCompile(`master offset\s+(-?\d+)`)
	lines := strings.Split(logOutput, "\n")
	// Search from the last line backwards to find the most recent offset
	for i := len(lines) - 1; i >= 0; i-- {
		matches := re.FindStringSubmatch(lines[i])
		if len(matches) >= 2 {
			return strconv.ParseFloat(matches[1], 64)
		}
	}
	return 0, &parseError{"no master offset found in ptp4l log"}
}

// getChronycOffset retrieves the NTP offset from chronyc tracking.
func getChronycOffset() (float64, error) {
	out, err := exec.Command("chronyc", "tracking").Output()
	if err != nil {
		return 0, err
	}
	return parseChronycOffset(string(out))
}

// parseChronycOffset extracts "Last offset" from chronyc tracking output.
// Example line: "Last offset     : -0.000012345 seconds"
func parseChronycOffset(output string) (float64, error) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Last offset") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}
			fields := strings.Fields(parts[1])
			if len(fields) < 1 {
				continue
			}
			seconds, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				return 0, err
			}
			// Convert seconds to nanoseconds
			return seconds * 1e9, nil
		}
	}
	return 0, &parseError{"no Last offset found in chronyc output"}
}

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	return e.msg
}
