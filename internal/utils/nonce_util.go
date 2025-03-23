package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateNonce generates a cryptographically secure random nonce.
// If crypto/rand fails, it falls back to using timestamp and UUID.
func GenerateNonce() string {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		nonceBytes = []byte(fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String()))
	}
	return hex.EncodeToString(nonceBytes)
}

// VerifyNonce checks if a nonce exists in a given output string
func VerifyNonce(taskNonce string, taskOutput string) bool {
	if taskNonce == "" || taskOutput == "" {
		return false
	}
	return strings.Contains(taskOutput, taskNonce)
}
