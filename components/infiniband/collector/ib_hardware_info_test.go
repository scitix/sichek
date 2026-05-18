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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinLinkCurSpeed(t *testing.T) {
	cases := []struct {
		name    string
		links   []PCIETreeLink
		wantVal string
		wantBDF string
	}{
		{
			name:    "empty",
			links:   nil,
			wantVal: "",
			wantBDF: "",
		},
		{
			name: "single_link",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: "32.0 GT/s PCIe"},
			},
			wantVal: "32.0 GT/s PCIe",
			wantBDF: "b",
		},
		{
			name: "lowest_wins_returns_child_bdf",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: "32.0 GT/s PCIe"},
				{ParentBDF: "b", ChildBDF: "c", CurSpeed: "16.0 GT/s PCIe"},
				{ParentBDF: "c", ChildBDF: "d", CurSpeed: "8.0 GT/s PCIe"},
			},
			wantVal: "8.0 GT/s PCIe",
			wantBDF: "d",
		},
		{
			name: "blank_speeds_skipped",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: ""},
				{ParentBDF: "b", ChildBDF: "c", CurSpeed: "16.0 GT/s PCIe"},
			},
			wantVal: "16.0 GT/s PCIe",
			wantBDF: "c",
		},
		{
			name: "all_blank",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: ""},
			},
			wantVal: "",
			wantBDF: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, bdf := minLinkCurSpeed(tc.links)
			assert.Equal(t, tc.wantVal, val)
			assert.Equal(t, tc.wantBDF, bdf)
		})
	}
}

func TestMinLinkCurWidth(t *testing.T) {
	cases := []struct {
		name    string
		links   []PCIETreeLink
		wantVal string
		wantBDF string
	}{
		{
			name:    "empty",
			links:   nil,
			wantVal: "",
			wantBDF: "",
		},
		{
			name: "lowest_wins",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurWidth: "16"},
				{ParentBDF: "b", ChildBDF: "c", CurWidth: "8"},
			},
			wantVal: "8",
			wantBDF: "c",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, bdf := minLinkCurWidth(tc.links)
			assert.Equal(t, tc.wantVal, val)
			assert.Equal(t, tc.wantBDF, bdf)
		})
	}
}
