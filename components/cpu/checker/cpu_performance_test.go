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
package checker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGovernors builds cpu<N>/cpufreq/scaling_governor files under a temp dir
// and points governorGlob at them. modes == nil leaves no cpufreq at all (VM case).
func fakeGovernors(t *testing.T, modes []string) string {
	t.Helper()
	root := t.TempDir()
	for i, m := range modes {
		dir := filepath.Join(root, "cpu"+strconv.Itoa(i), "cpufreq")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "scaling_governor"), []byte(m+"\n"), 0644))
	}
	old := governorGlob
	governorGlob = filepath.Join(root, "cpu*", "cpufreq", "scaling_governor")
	t.Cleanup(func() { governorGlob = old })
	return root
}

func readGovernor(t *testing.T, root string, cpu int) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "cpu"+strconv.Itoa(cpu), "cpufreq", "scaling_governor"))
	require.NoError(t, err)
	return string(b)
}

func TestCPUPerfChecker_NoCpufreq(t *testing.T) {
	fakeGovernors(t, nil)
	c, err := NewCPUPerfChecker()
	require.NoError(t, err)

	res, err := c.Check(context.Background(), nil)
	require.NoError(t, err, "missing cpufreq must not be a checker error")
	assert.Equal(t, consts.StatusNormal, res.Status)
	assert.Equal(t, "N/A", res.Curr)
	assert.Contains(t, res.Detail, "cpufreq not available")
}

func TestCPUPerfChecker_AllPerformance(t *testing.T) {
	fakeGovernors(t, []string{"performance", "performance"})
	c, _ := NewCPUPerfChecker()

	res, err := c.Check(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, res.Status)
	assert.Equal(t, "Enabled", res.Curr)
}

func TestCPUPerfChecker_FixedToPerformance(t *testing.T) {
	root := fakeGovernors(t, []string{"powersave", "performance"})
	c, _ := NewCPUPerfChecker()

	res, err := c.Check(context.Background(), nil)
	require.NoError(t, err)
	// The checker switched cpu0 to performance successfully: that is a pass, not a failure.
	assert.Equal(t, "performance", readGovernor(t, root, 0))
	assert.Equal(t, consts.StatusNormal, res.Status)
	assert.Equal(t, "Enabled", res.Curr)
	assert.Contains(t, res.Detail, "successfully")
}
