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
	"strconv"
	"strings"
)

// ParseMetrics parses a Prometheus text-exposition body into series grouped by metric
// name, keeping only series whose name starts with prefix. It mirrors rdma-env-pre's
// own renderer (a name{labels} value line, optional trailing timestamp); # HELP/# TYPE
// and blank lines are skipped, and a malformed line is dropped rather than failing the
// whole parse.
func ParseMetrics(body, prefix string) map[string][]Series {
	out := make(map[string][]Series)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, value, ok := parseLine(line)
		if !ok {
			continue
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		out[name] = append(out[name], Series{Name: name, Labels: labels, Value: value})
	}
	return out
}

// parseLine parses one exposition line into name, labels, and value.
func parseLine(line string) (name string, labels map[string]string, value float64, ok bool) {
	labels = map[string]string{}
	var rest string

	i := strings.IndexAny(line, "{ ")
	if i < 0 {
		return "", nil, 0, false // no value present
	}
	if line[i] == '{' {
		name = line[:i]
		closeIdx := findLabelClose(line, i)
		if closeIdx < 0 {
			return "", nil, 0, false
		}
		labels = parseLabels(line[i+1 : closeIdx])
		rest = strings.TrimSpace(line[closeIdx+1:])
	} else {
		name = line[:i]
		rest = strings.TrimSpace(line[i+1:])
	}

	fields := strings.Fields(rest) // value [timestamp]; timestamp ignored
	if len(fields) == 0 {
		return "", nil, 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", nil, 0, false
	}
	if name == "" {
		return "", nil, 0, false
	}
	return name, labels, v, true
}

// findLabelClose returns the index of the '}' that closes the label set opened at
// open, skipping any '}' that appears inside a quoted label value.
func findLabelClose(line string, open int) int {
	inQuote := false
	esc := false
	for i := open + 1; i < len(line); i++ {
		c := line[i]
		if inQuote {
			if esc {
				esc = false
				continue
			}
			switch c {
			case '\\':
				esc = true
			case '"':
				inQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
		case '}':
			return i
		}
	}
	return -1
}

// parseLabels parses a k="v",k2="v2" label body into a map, unescaping values.
func parseLabels(s string) map[string]string {
	m := map[string]string{}
	for _, pair := range splitTopLevel(s) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		rawKey, rawVal, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		k := strings.TrimSpace(rawKey)
		v := strings.TrimSpace(rawVal)
		v = strings.TrimPrefix(v, `"`)
		v = strings.TrimSuffix(v, `"`)
		if k == "" {
			continue
		}
		m[k] = unescape(v)
	}
	return m
}

// splitTopLevel splits on commas that are not inside a quoted value.
func splitTopLevel(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			cur.WriteByte(c)
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
			} else if c == '"' {
				inQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
			cur.WriteByte(c)
		case ',':
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	parts = append(parts, cur.String())
	return parts
}

// unescape reverses Prometheus label-value escaping: \\ -> \, \" -> ", \n -> newline.
func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
			case '"':
				b.WriteByte('"')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
