package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/config"
	"github.com/theblitlabs/parity-server/internal/utils"
)

const (
	KeystoreDirName  = ".parity"
	KeystoreFileName = "keystore.json"
)

func RunAuth() {
	var privateKey string
	logger := log.With().Str("component", "auth").Logger()

	cmd := utils.CreateCommand(utils.CommandConfig{
		Use:   "auth",
		Short: "Authenticate with the network",
		Flags: map[string]utils.Flag{
			"private-key": {
				Type:        utils.FlagTypeString,
				Shorthand:   "k",
				Description: "Private key in hex format",
				Required:    true,
			},
		},
		RunFunc: func(cmd *cobra.Command, args []string) error {
			var err error
			privateKey, err = cmd.Flags().GetString("private-key")
			if err != nil {
				return fmt.Errorf("failed to get private key flag: %w", err)
			}

			return ExecuteAuth(privateKey, "config/config.yaml")
		},
	}, logger)

	utils.ExecuteCommand(cmd, logger)
}

func ExecuteAuth(privateKey string, configPath string) error {
	log := log.With().Str("component", "auth").Logger()

	if privateKey == "" {
		return fmt.Errorf("private key is required")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	privateKey = strings.TrimPrefix(privateKey, "0x")

	if len(privateKey) != 64 {
		return fmt.Errorf("invalid private key - must be 64 hex characters without 0x prefix")
	}

	_, err = crypto.HexToECDSA(privateKey)
	if err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, KeystoreDirName),
		FileName: KeystoreFileName,
	})
	if err != nil {
		return fmt.Errorf("failed to create keystore: %w", err)
	}

	if err := ks.SavePrivateKey(privateKey); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	tokenAddress := common.HexToAddress(cfg.Ethereum.TokenAddress)
	client, err := walletsdk.NewClient(walletsdk.ClientConfig{
		RPCURL:       cfg.Ethereum.RPC,
		ChainID:      cfg.Ethereum.ChainID,
		PrivateKey:   privateKey,
		TokenAddress: tokenAddress,
	})
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	log.Info().
		Str("address", client.Address().Hex()).
		Str("keystore", fmt.Sprintf("%s/%s", KeystoreDirName, KeystoreFileName)).
		Msg("Wallet authenticated successfully")

	return nil
}
