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
