package keystore

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/theblitlabs/parity-server/pkg/logger"
)

type Keystore struct {
	AuthToken string `json:"auth_token"`
	CreatedAt int64  `json:"created_at"`
}

type KeyStore struct {
	PrivateKey string `json:"private_key"`
}

func GetKeystorePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	keystoreDir := filepath.Join(homeDir, ".parity")
	if err := os.MkdirAll(keystoreDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create keystore directory: %w", err)
	}

	return filepath.Join(keystoreDir, "keystore.json"), nil
}

func SaveToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	keystorePath, err := GetKeystorePath()
	if err != nil {
		return err
	}

	keystore := Keystore{
		AuthToken: token,
		CreatedAt: time.Now().Unix(),
	}

	log := logger.Get()
	log.Info().
		Str("path", keystorePath).
		Str("token_preview", token[:min(len(token), 10)]+"...").
		Msg("Saving token to keystore")

	data, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keystore: %w", err)
	}

	if err := os.WriteFile(keystorePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write keystore file: %w", err)
	}

	savedToken, err := LoadToken()
	if err != nil {
		return fmt.Errorf("token verification failed after save: %w", err)
	}
	if savedToken != token {
		return fmt.Errorf("token verification mismatch after save - Original: %s, Saved: %s",
			token[:10], savedToken[:10])
	}

	return nil
}

func LoadToken() (string, error) {
	keystorePath, err := GetKeystorePath()
	if err != nil {
		return "", err
	}

	log := logger.Get()
	log.Info().
		Str("path", keystorePath).
		Msg("Loading token from keystore")

	data, err := os.ReadFile(keystorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no keystore found at %s - please authenticate first", keystorePath)
		}
		return "", fmt.Errorf("failed to read keystore: %w", err)
	}

	var keystore Keystore
	if err := json.Unmarshal(data, &keystore); err != nil {
		return "", fmt.Errorf("failed to parse keystore: %w", err)
	}

	if time.Now().Unix()-keystore.CreatedAt > 3600 {
		return "", fmt.Errorf("token has expired - please re-authenticate")
	}

	if keystore.AuthToken == "" {
		return "", fmt.Errorf("invalid token found in keystore")
	}

	tokenAge := time.Now().Unix() - keystore.CreatedAt
	log.Info().
		Str("length", fmt.Sprintf("%d", len(keystore.AuthToken))).
		Str("token_preview", keystore.AuthToken[:10]+"...").
		Str("age_seconds", fmt.Sprintf("%d", tokenAge)).
		Msg("Token loaded successfully")

	return keystore.AuthToken, nil
}

func LoadPrivateKey() (*ecdsa.PrivateKey, error) {
	keystorePath, err := GetKeystorePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(keystorePath)
	if err != nil {
		return nil, err
	}

	// Parse the JSON structure
	var ks struct {
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &ks); err != nil {
		return nil, fmt.Errorf("invalid keystore format: %w", err)
	}

	return crypto.HexToECDSA(ks.PrivateKey)
}

func SavePrivateKey(privateKeyHex string) error {
	keystorePath, err := GetKeystorePath()
	if err != nil {
		return err
	}

	// Validate private key format
	if _, err := crypto.HexToECDSA(privateKeyHex); err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}

	return os.WriteFile(keystorePath, []byte(privateKeyHex), 0600)
}

func GetPrivateKey() (string, error) {
	keystorePath := filepath.Join(os.Getenv("HOME"), ".parity", "keystore.json")

	data, err := os.ReadFile(keystorePath)
	if err != nil {
		return "", fmt.Errorf("failed to read keystore: %w", err)
	}

	// Parse the JSON structure
	var ks struct {
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &ks); err != nil {
		return "", fmt.Errorf("invalid keystore format - try re-authenticating: %w", err)
	}

	if ks.PrivateKey == "" {
		return "", fmt.Errorf("no private key found in keystore")
	}

	return ks.PrivateKey, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
