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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGPUNumaNode(t *testing.T) {
	root := t.TempDir()
	orig := PCIDevicesPath
	PCIDevicesPath = root
	t.Cleanup(func() { PCIDevicesPath = orig })

	writeNuma := func(bdf, content string) {
		dir := filepath.Join(root, bdf)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "numa_node"), []byte(content), 0o644))
	}
	writeNuma("0000:1b:00.0", "1\n")
	writeNuma("0000:9a:00.0", "-1\n")

	tests := []struct {
		name    string
		bdf     string
		want    string
		wantErr bool
	}{
		{name: "numa node 1", bdf: "0000:1b:00.0", want: "1"},
		{name: "upper-case bdf is normalized", bdf: "0000:1B:00.0", want: "1"},
		{name: "no numa affinity", bdf: "0000:9a:00.0", want: "-1"},
		{name: "missing device", bdf: "0000:ff:00.0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetGPUNumaNode(tt.bdf)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
