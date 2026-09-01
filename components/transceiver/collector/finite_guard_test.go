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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFiniteOrZero(t *testing.T) {
	assert.Equal(t, 0.0, finiteOrZero(math.Inf(1)))
	assert.Equal(t, 0.0, finiteOrZero(math.Inf(-1)))
	assert.Equal(t, 0.0, finiteOrZero(math.NaN()))
	assert.Equal(t, -1.5, finiteOrZero(-1.5))
	assert.Equal(t, 0.0, finiteOrZero(0.0))
}

// A dark / unconnected optical lane reports "-inf dBm"; strconv.ParseFloat
// accepts "-inf" as a valid float, so without the guard the power reading would
// be -Inf and later break the whole snapshot's JSON marshal.
func TestParseDBM_NonFinite(t *testing.T) {
	assert.Equal(t, 0.0, parseDBM("-inf dBm"))
	assert.Equal(t, 0.0, parseDBM("inf dBm"))
	// Normal readings still parse, including the "high/low" slash form.
	assert.Equal(t, -1.5, parseDBM("-1.5 dBm"))
	assert.Equal(t, -2.3, parseDBM("-40.0 / -2.3 dBm"))
}

func TestParseMLXLinkBER_NonFinite(t *testing.T) {
	assert.Equal(t, 0.0, parseMLXLinkBER("-inf"))
	assert.Equal(t, 0.0, parseMLXLinkBER("inf"))
	assert.Equal(t, 0.0, parseMLXLinkBER("nan"))
	assert.Equal(t, 0.0, parseMLXLinkBER("N/A"))
	// Scientific-notation BER values still parse.
	assert.InDelta(t, 2e-9, parseMLXLinkBER("2E-9"), 1e-20)
	assert.InDelta(t, 15e-255, parseMLXLinkBER("15E-255"), 1e-260)
}

func TestParseMLXLinkMultiLane_NonFinite(t *testing.T) {
	// A lane reporting -inf is normalized to 0 while finite lanes are kept.
	got := parseMLXLinkMultiLane("-inf,8.380,8.380 [2..15]")
	assert.Equal(t, []float64{0, 8.380, 8.380}, got)
}
