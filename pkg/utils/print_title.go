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
package utils

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func GetTerminalWidth() (int, error) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Println("Error getting terminal size:", err)
		return 0, err
	}
	return int(float64(width) * 0.8), nil
}

func PrintTitle(text, paddingChar string) {
	width, err := GetTerminalWidth()
	if err != nil {
		return
	}

	textLength := len(text)
	paddingLength := width - textLength
	if paddingLength < 0 {
		fmt.Println(text)
	}
	leftPadding := paddingLength / 2
	rightPadding := paddingLength - leftPadding

	left := strings.Repeat(paddingChar, leftPadding/len(paddingChar)) + paddingChar[:leftPadding%len(paddingChar)]
	right := strings.Repeat(paddingChar, rightPadding/len(paddingChar)) + paddingChar[:rightPadding%len(paddingChar)]
	fmt.Println(left + text + right)
}
