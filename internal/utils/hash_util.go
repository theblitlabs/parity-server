package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashDeviceID generates a SHA-256 hash of a device ID
func HashDeviceID(deviceID string) string {
	hash := sha256.Sum256([]byte(deviceID))
	return hex.EncodeToString(hash[:])
}
