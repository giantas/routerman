package cli

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var macAddressRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)

func IsValidMacAddress(value string) bool {
	return macAddressRegex.MatchString(value)
}

func GetInput(in io.Reader) (string, error) {
	reader := bufio.NewReader(in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)
	return input, nil
}

func GetChoiceInput(in io.Reader, max int) (int, error) {
	input, err := GetInput(in)
	if err != nil {
		return 0, err
	}
	if ToLowerCaseChar(input) == "b" {
		return ExitChoice, err
	}
	if ToLowerCaseChar(input) == "q" {
		return QuitChoice, err
	}
	return GetChoice(input, max)
}

func GetIntInput(in io.Reader, defaultValue int) (int, error) {
	input, err := GetInput(in)
	if input == "" {
		return defaultValue, err
	}
	num, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("invalid input")
	}
	return num, err
}

func GetChoice(value string, max int) (int, error) {
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0, ErrInvalidChoice
	}
	if num < 1 || num > max {
		return 0, ErrInvalidChoice
	}
	return num - 1, err
}

func ToLowerCaseChar(char string) string {
	r := []rune(char)
	if len(r) == 0 {
		return char
	}
	firstChar := unicode.ToLower(r[0])
	if len(r) == 1 {
		return string(firstChar)
	}
	return string(firstChar) + string(r[0:])
}
