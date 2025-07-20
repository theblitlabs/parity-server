package services

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

const (
	KeystoreDirName  = ".parity"
	KeystoreFileName = "keystore.json"
)

type BlockchainRewardClient struct {
	cfg *config.Config
}

func NewBlockchainRewardClient(cfg *config.Config) *BlockchainRewardClient {
	return &BlockchainRewardClient{
		cfg: cfg,
	}
}

// SetStakeWallet is deprecated - reward distribution now uses real blockchain transactions only

func (c *BlockchainRewardClient) DistributeRewards(result *models.TaskResult) error {
	log := gologger.WithComponent("rewards").With().
		Str("task", result.TaskID.String()).
		Str("device", result.DeviceID).
		Logger()

	log.Info().Msg("Starting reward distribution")

	if result.Reward <= 0 {
		log.Error().Msg("Invalid reward amount - must be greater than zero")
		return fmt.Errorf("invalid reward amount: must be greater than zero")
	}

	// Always use real wallet implementation - no more mock transfers

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get home directory")
		return fmt.Errorf("home directory error: %w", err)
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, KeystoreDirName),
		FileName: KeystoreFileName,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create keystore")
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Auth required")
		return fmt.Errorf("auth required: %w", err)
	}

	client, err := walletsdk.NewClient(walletsdk.ClientConfig{
		RPCURL:       c.cfg.BlockchainNetwork.RPC,
		ChainID:      int64(c.cfg.BlockchainNetwork.ChainID),
		TokenAddress: common.HexToAddress(c.cfg.BlockchainNetwork.TokenAddress),
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	})
	if err != nil {
		log.Error().Err(err).Msg("Client creation failed")
		return fmt.Errorf("wallet client failed: %w", err)
	}

	log.Debug().
		Str("wallet", client.Address().Hex()).
		Str("rpc", c.cfg.BlockchainNetwork.RPC).
		Int64("chain", c.cfg.BlockchainNetwork.ChainID).
		Msg("Client initialized")

	stakeWalletAddr := common.HexToAddress(c.cfg.BlockchainNetwork.StakeWalletAddress)
	stakeWallet, err := walletsdk.NewStakeWallet(
		client,
		stakeWalletAddr,
		common.HexToAddress(c.cfg.BlockchainNetwork.TokenAddress),
	)
	if err != nil {
		log.Error().Err(err).
			Str("addr", stakeWalletAddr.Hex()).
			Msg("Contract init failed")
		return fmt.Errorf("stake wallet init failed: %w", err)
	}

	stakeInfo, err := stakeWallet.GetStakeInfo(result.DeviceID)
	if err != nil {
		log.Error().Err(err).Msg("Stake info check failed")
		return nil
	}

	if !stakeInfo.Exists {
		log.Debug().Msg("No stake found")
		return nil
	}

	log.Debug().Str("amount", stakeInfo.Amount.String()).Msg("Found stake")

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	log.Debug().
		Str("reward", rewardAmount.String()).
		Str("creator", result.CreatorDeviceID).
		Msg("Initiating transfer")

	tx, err := stakeWallet.TransferPayment(
		result.CreatorDeviceID,
		result.DeviceID,
		rewardAmount,
	)
	if err != nil {
		log.Error().Err(err).
			Str("reward", rewardAmount.String()).
			Str("creator", result.CreatorDeviceID).
			Msg("Transfer failed")
		return fmt.Errorf("reward transfer failed: %w", err)
	}

	log.Info().
		Str("tx", tx.Hash().Hex()).
		Str("reward", rewardAmount.String()).
		Msg("Transfer submitted")

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().Err(err).
			Str("tx", tx.Hash().Hex()).
			Msg("Confirmation failed")
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if receipt.Status == 0 {
		log.Error().
			Str("tx", tx.Hash().Hex()).
			Str("reward", rewardAmount.String()).
			Msg("Transfer reverted")
		return fmt.Errorf("transfer reverted")
	}

	log.Info().
		Str("tx", tx.Hash().Hex()).
		Str("reward", rewardAmount.String()).
		Str("block", receipt.BlockNumber.String()).
		Msg("Transfer confirmed")

	return nil
}

// Removed mock wallet distribution function - all transfers are now real blockchain transactions
