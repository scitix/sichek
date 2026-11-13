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
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serverModelSeries(t *testing.T) []*dto.Metric {
	t.Helper()
	ch := make(chan prometheus.Metric, 10)
	serverModelInfo.Collect(ch)
	close(ch)
	var out []*dto.Metric
	for m := range ch {
		var d dto.Metric
		require.NoError(t, m.Write(&d))
		out = append(out, &d)
	}
	return out
}

func labelValue(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

func TestSetServerModelInfo_ReplacesPreviousSeries(t *testing.T) {
	SetServerModelInfo("I57G90N13M63", false)
	SetServerModelInfo("I57G90N13M63S10", true)

	series := serverModelSeries(t)
	require.Len(t, series, 1)
	assert.Equal(t, "I57G90N13M63S10", labelValue(series[0], "server_model_id"))
	assert.Equal(t, "true", labelValue(series[0], "complete"))
	assert.NotEmpty(t, labelValue(series[0], "node"))
	assert.Equal(t, 1.0, series[0].GetGauge().GetValue())
}
