package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

// maxLoggedJSONBytes caps request/response JSON written to the stderr log.
// Payloads can embed base64 data (e.g. rendered images) that would otherwise
// produce single log lines of many megabytes, which line-based stderr capture
// in the host cannot handle.
const maxLoggedJSONBytes = 10 * 1024

// truncatedJSON returns PrettyJSON(v) capped at maxLoggedJSONBytes.
func truncatedJSON(v interface{}) string {
	s := PrettyJSON(v)
	if len(s) <= maxLoggedJSONBytes {
		return s
	}
	return fmt.Sprintf("%s\n... [truncated %d of %d bytes]", s[:maxLoggedJSONBytes], len(s)-maxLoggedJSONBytes, len(s))
}

// PrettyJSON takes any value and returns a formatted JSON string representation.
// If the input cannot be marshaled to JSON, it returns an error.
func PrettyJSON(v interface{}) string {
	// First marshal the object to JSON
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		log.Printf("failed to marshal to JSON: %v", err)
		return ""
	}

	// Create a buffer for pretty printing
	var prettyJSON bytes.Buffer

	// Use json.Indent to format the JSON with standard indentation
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		log.Printf("failed to indent JSON: %v", err)
		return ""
	}

	return prettyJSON.String()
}
