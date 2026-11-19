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
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestShouldLookupGateway(t *testing.T) {
	tests := []struct {
		name       string
		hasV4      bool
		v4Err      error
		wantLookup bool
		wantLog    string
		noLog      []string
	}{
		{
			// Multi-plane CX8 plane ports (e.g. eth_r2_p0) carry only fe80:: and no IPv4.
			name:       "no IPv4 skips lookup quietly",
			hasV4:      false,
			wantLookup: false,
			noLog:      []string{"level=error", "level=warning", "has IPv4 address"},
		},
		{
			name:       "IPv4 present continues lookup",
			hasV4:      true,
			wantLookup: true,
			wantLog:    "has IPv4 address",
			noLog:      []string{"level=error"},
		},
		{
			name:       "IPv4 probe error is logged and skips lookup",
			v4Err:      assert.AnError,
			wantLookup: false,
			wantLog:    "level=error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := ifaceHasIPv4
			ifaceHasIPv4 = func(string) (bool, error) { return tt.hasV4, tt.v4Err }
			t.Cleanup(func() { ifaceHasIPv4 = old })

			var buf bytes.Buffer
			oldOut, oldLevel := logrus.StandardLogger().Out, logrus.GetLevel()
			logrus.SetOutput(&buf)
			logrus.SetLevel(logrus.InfoLevel)
			t.Cleanup(func() { logrus.SetOutput(oldOut); logrus.SetLevel(oldLevel) })

			got := shouldLookupGateway("eth_r2_p0", "roce_r2")
			assert.Equal(t, tt.wantLookup, got)
			if tt.wantLog != "" {
				assert.Contains(t, buf.String(), tt.wantLog)
			}
			for _, s := range tt.noLog {
				assert.NotContains(t, buf.String(), s)
			}
		})
	}
}
