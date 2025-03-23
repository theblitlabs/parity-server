package services

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type EthereumRewardClient struct {
	cfg         *config.Config
	stakeWallet ports.StakeWallet
}

func NewEthereumRewardClient(cfg *config.Config) *EthereumRewardClient {
	return &EthereumRewardClient{
		cfg: cfg,
	}
}

func (c *EthereumRewardClient) SetStakeWallet(sw ports.StakeWallet) {
	c.stakeWallet = sw
}

func (c *EthereumRewardClient) DistributeRewards(result *models.TaskResult) error {
	log := gologger.WithComponent("rewards").With().
		Str("task", result.TaskID.String()).
		Str("device", result.DeviceID).
		Logger()

	log.Info().Msg("Starting reward distribution")

	if result.Reward <= 0 {
		log.Error().Msg("Invalid reward amount - must be greater than zero")
		return fmt.Errorf("invalid reward amount: must be greater than zero")
	}

	if c.stakeWallet != nil {
		return c.distributeWithMockWallet(log, result)
	}

	ks, err := keystore.NewKeystore(keystore.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create keystore")
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Auth required")
		return fmt.Errorf("auth required: %w", err)
	}

	client, err := walletsdk.NewClient(walletsdk.ClientConfig{
		RPCURL:       c.cfg.Ethereum.RPC,
		ChainID:      int64(c.cfg.Ethereum.ChainID),
		TokenAddress: common.HexToAddress(c.cfg.Ethereum.TokenAddress),
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	})
	if err != nil {
		log.Error().Err(err).Msg("Client creation failed")
		return fmt.Errorf("wallet client failed: %w", err)
	}

	log.Debug().
		Str("wallet", client.Address().Hex()).
		Str("rpc", c.cfg.Ethereum.RPC).
		Int64("chain", c.cfg.Ethereum.ChainID).
		Msg("Client initialized")

	stakeWalletAddr := common.HexToAddress(c.cfg.Ethereum.StakeWalletAddress)
	stakeWallet, err := walletsdk.NewStakeWallet(
		client,
		stakeWalletAddr,
		common.HexToAddress(c.cfg.Ethereum.TokenAddress),
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

// distributeWithMockWallet handles reward distribution using a mock wallet for testing
func (c *EthereumRewardClient) distributeWithMockWallet(log zerolog.Logger, result *models.TaskResult) error {
	stakeInfo, err := c.stakeWallet.GetStakeInfo(result.DeviceID)
	if err != nil {
		log.Error().Err(err).Msg("Stake info check failed")
		return nil
	}

	if !stakeInfo.Exists {
		log.Debug().Msg("No stake found")
		return nil
	}

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	if err := c.stakeWallet.TransferPayment(result.CreatorAddress, result.DeviceID, rewardAmount); err != nil {
		log.Error().Err(err).
			Str("reward", rewardAmount.String()).
			Msg("Transfer failed")
		return fmt.Errorf("reward transfer failed: %w", err)
	}

	log.Info().Str("reward", rewardAmount.String()).Msg("Transfer completed")
	return nil
}