package main

import "regexp"

var macAddressRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)

func IsValidMacAddress(value string) bool {
	return macAddressRegex.MatchString(value)
}
