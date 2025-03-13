package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
)

type NonceService struct{}

func NewNonceService() *NonceService {
	return &NonceService{}
}

func (s *NonceService) GenerateNonce() string {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		nonceBytes = []byte(fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String()))
	}
	return hex.EncodeToString(nonceBytes)
}

func (s *NonceService) VerifyNonce(taskNonce string, taskOutput string) bool {
	if taskNonce == "" || taskOutput == "" {
		return false
	}

	log := gologger.WithComponent("nonce_service")
	log.Debug().
		Str("nonce", taskNonce).
		Str("output_length", fmt.Sprintf("%d", len(taskOutput))).
		Msg("Verifying nonce in task output")

	return strings.Contains(taskOutput, taskNonce)
}
