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
	"testing"
)

func TestMemoryInfo_Get(t *testing.T) {
	// Mock /proc/meminfo content
	mockMemInfo := `
MemTotal:       2113413736 kB
MemFree:        1926759156 kB
MemAvailable:   2057833168 kB
Buffers:         2620172 kB
Cached:         132259752 kB
SwapCached:            0 kB
Active:         18719820 kB
Inactive:       118736920 kB
Active(anon):      30860 kB
Inactive(anon):  2570524 kB
Active(file):   18688960 kB
Inactive(file): 116166396 kB
Unevictable:    33582048 kB
Mlocked:        33582048 kB
SwapTotal:             0 kB
SwapFree:              0 kB
Dirty:               172 kB
Writeback:            52 kB
AnonPages:      36154820 kB
Mapped:          1348808 kB
Shmem:             30612 kB
KReclaimable:    6754380 kB
Slab:            8973176 kB
SReclaimable:    6754380 kB
SUnreclaim:      2218796 kB
KernelStack:       87792 kB
PageTables:       111924 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:    1056706868 kB
Committed_AS:   45725800 kB
VmallocTotal:   13743895347199 kB
VmallocUsed:     1161424 kB
VmallocChunk:          0 kB
Percpu:           631296 kB
HardwareCorrupted:     0 kB
AnonHugePages:     65536 kB
ShmemHugePages:        0 kB
ShmemPmdMapped:        0 kB
FileHugePages:         0 kB
FilePmdMapped:         0 kB
HugePages_Total:       0
HugePages_Free:        0
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       2048 kB
Hugetlb:               0 kB
DirectMap4k:     9564424 kB
DirectMap2M:    459855872 kB
DirectMap1G:    1679818752 kB
`

	// Create a temporary file to simulate /proc/meminfo
	tmpFile, err := os.CreateTemp("", "meminfo")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(mockMemInfo)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	// Create a MemoryInfo instance and call the Get method
	var memInfo MemoryInfo
	err = memInfo.get(tmpFile.Name())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify the results
	if memInfo.MemTotal != 2113413736*1024 {
		t.Errorf("MemTotal = %d, want %d", memInfo.MemTotal, 2113413736*1024)
	}
	if memInfo.MemFree != 1926759156*1024 {
		t.Errorf("MemFree = %d, want %d", memInfo.MemFree, 1926759156*1024)
	}
	if memInfo.MemUsed != (2113413736-1926759156)*1024 {
		t.Errorf("MemUsed = %d, want %d", memInfo.MemUsed, (2113413736-1926759156)*1024)
	}
	if memInfo.MemPercentUsed != ((2113413736-1926759156)*100)/2113413736 {
		t.Errorf("MemPercentUsed = %d, want %d", memInfo.MemPercentUsed, ((2113413736-1926759156)*100)/2113413736)
	}
	if memInfo.MemAnonymousUsed != 36154820*1024 {
		t.Errorf("MemAnonymousUsed = %d, want %d", memInfo.MemAnonymousUsed, 36154820*1024)
	}
	if memInfo.PageCacheUsed != 132259752*1024 {
		t.Errorf("PageCacheUsed = %d, want %d", memInfo.PageCacheUsed, 132259752*1024)
	}
	if memInfo.MemUnevictableUsed != 33582048*1024 {
		t.Errorf("MemUnevictableUsed = %d, want %d", memInfo.MemUnevictableUsed, 33582048*1024)
	}
	if memInfo.DirtyPageUsed != 172*1024 {
		t.Errorf("DirtyPageUsed = %d, want %d", memInfo.DirtyPageUsed, 172*1024)
	}
	t.Logf("MemoryInfo: %s", memInfo.ToString())
}
