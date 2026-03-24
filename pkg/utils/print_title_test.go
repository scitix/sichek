package utils

import (
	"testing"
)

func TestPrintTitle(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		paddingChar string
	}{
		{
			name:        "Normal case",
			text:        "hello",
			paddingChar: "-",
		},
		{
			name:        "Narrow terminal",
			text:        "this is a very long text that exceeds terminal width",
			paddingChar: "=",
		},
		{
			name:        "Empty padding char",
			text:        "test",
			paddingChar: "",
		},
		{
			name:        "Multi-byte text",
			text:        "你好世界",
			paddingChar: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We mainly want to ensure it doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PrintTitle panicked: %v", r)
				}
			}()
			PrintTitle(tt.text, tt.paddingChar)
		})
	}
}
