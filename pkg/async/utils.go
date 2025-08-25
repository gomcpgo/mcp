package async

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID creates a simple 8-character random ID
func generateID() string {
	bytes := make([]byte, 4) // 4 bytes = 8 hex characters
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return hex.EncodeToString([]byte{byte(timeNow().Unix() & 0xFF)})[:8]
	}
	return hex.EncodeToString(bytes)
}

// For testing - allows mocking time
var timeNow = defaultTimeNow

func defaultTimeNow() timeInterface {
	return realTime{}
}