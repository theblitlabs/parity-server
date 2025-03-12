package wallet

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/theblitlabs/parity-server/internal/config"
	"github.com/theblitlabs/parity-server/pkg/keystore"
	"github.com/theblitlabs/parity-server/pkg/logger"
)

// Define custom errors
var (
	ErrInvalidInfuraKey = fmt.Errorf("invalid infura project id")
	ErrNoAuthToken      = fmt.Errorf("no authentication token")
	ErrInvalidAuthToken = fmt.Errorf("invalid authentication token")
)

func CheckWalletConnection(cfg *config.Config) error {
	// Create wallet client
	client, err := NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		if err.Error() == "401 Unauthorized: invalid project id" {
			return ErrInvalidInfuraKey
		}
		return fmt.Errorf("failed to create ethereum client: %w", err)
	}

	// Get keystore path first
	keystorePath, err := keystore.GetKeystorePath()
	if err != nil {
		return fmt.Errorf("failed to get keystore path: %w", err)
	}

	// Load token from keystore
	authToken, err := keystore.LoadToken()
	if err != nil {
		if os.IsNotExist(err) ||
			err.Error() == fmt.Sprintf("no keystore found at %s - please authenticate first", keystorePath) {
			return ErrNoAuthToken
		}
		return fmt.Errorf("failed to load auth token: %w", err)
	}

	// Verify token and get wallet address
	claims, err := VerifyToken(authToken)
	if err != nil {
		return ErrInvalidAuthToken
	}

	// Get wallet address from token claims
	walletAddress := common.HexToAddress(claims.Address)

	// Check ERC20 token balance for the wallet address
	tokenContract := common.HexToAddress(cfg.Ethereum.TokenAddress)
	balance, err := client.GetERC20Balance(tokenContract, walletAddress)
	if err != nil {
		return fmt.Errorf("failed to check token balance: %w", err)
	}

	if balance.Sign() <= 0 {
		return fmt.Errorf("no tokens found in authenticated wallet %s", walletAddress.Hex())
	}

	log := logger.Get()
	log.Info().
		Str("wallet", walletAddress.Hex()).
		Str("token_contract", tokenContract.Hex()).
		Msg("Wallet authenticated successfully")

	return nil
}
