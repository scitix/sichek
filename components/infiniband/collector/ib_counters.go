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
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// IBCounters handles collection of InfiniBand counters
type IBCounters map[string]uint64

// Collect collects all counters for a given IB device and fills the map
func (cnt *IBCounters) Collect(IBDev string) {
	Counters := make(map[string]uint64, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex
	counterTypes := []string{"counters", "hw_counters"}

	wg.Add(len(counterTypes))
	for _, counterType := range counterTypes {
		go func(ct string) {
			defer wg.Done()
			counter, err := cnt.GetIBCounter(IBDev, ct)
			if err != nil {
				logrus.WithField("component", "infiniband").Errorf("Get IB Counter failed, err:%s", err)
				return
			}
			// Use mutex to protect concurrent writes to map
			mu.Lock()
			for k, v := range counter {
				Counters[k] = v
			}
			mu.Unlock()
		}(counterType)
	}
	wg.Wait()

	// Update the map
	for k, v := range Counters {
		(*cnt)[k] = v
	}
}

// GetIBCounter gets IB counter for a specific counter type
func (cnt *IBCounters) GetIBCounter(IBDev string, counterType string) (map[string]uint64, error) {
	Counters := make(map[string]uint64, 0)
	counterPath := path.Join(IBSYSPathPre, IBDev, "ports/1", counterType)
	ibCounterName, err := ListDir(counterPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Fail to get the counter from path :%s", counterPath)
		return nil, err
	}
	for _, counter := range ibCounterName {

		counterValuePath := path.Join(counterPath, counter)
		contents, err := os.ReadFile(counterValuePath)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Fail to read the ib counter from path: %s", counterValuePath)
		}
		// counter Value
		value, err := strconv.ParseUint(strings.ReplaceAll(string(contents), "\n", ""), 10, 64)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Error covering string to uint64")
			return nil, err
		}
		Counters[counter] = value
	}

	return Counters, nil
}
