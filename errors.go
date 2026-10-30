package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// print_errors.json is generated from Bambu Studio's device_error table via
// ha-bambulab's hms_en.json.gz. It maps an 8-hex print_error code to its
// English description.
//
//go:embed print_errors.json
var printErrorsJSON []byte

var printErrorMessages = loadPrintErrorMessages()

func loadPrintErrorMessages() map[string]string {
	messages := make(map[string]string)
	if err := json.Unmarshal(printErrorsJSON, &messages); err != nil {
		return map[string]string{}
	}
	return messages
}

func printErrorMessage(code int64) (string, string) {
	hexCode := fmt.Sprintf("%08X", code)
	message, ok := printErrorMessages[hexCode]
	if !ok {
		return hexCode, "Unknown print error"
	}
	return hexCode, message
}
