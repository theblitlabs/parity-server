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
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

// FLRewardService handles federated learning reward distribution
type FLRewardService struct {
	cfg               *config.Config
	flSessionRepo     ports.FLSessionRepository
	flParticipantRepo ports.FLParticipantRepository
	runnerService     ports.RunnerService
}

// NewFLRewardService creates a new FL reward service
func NewFLRewardService(
	cfg *config.Config,
	flSessionRepo ports.FLSessionRepository,
	flParticipantRepo ports.FLParticipantRepository,
	runnerService ports.RunnerService,
) *FLRewardService {
	return &FLRewardService{
		cfg:               cfg,
		flSessionRepo:     flSessionRepo,
		flParticipantRepo: flParticipantRepo,
		runnerService:     runnerService,
	}
}

// SetStakeWallet is deprecated - FL rewards now use real blockchain transactions only

// DistributeRoundRewards distributes rewards to participants after a FL round completion
func (s *FLRewardService) DistributeRoundRewards(ctx context.Context, sessionID, roundID uuid.UUID) error {
	log := gologger.WithComponent("fl_rewards").With().
		Str("session_id", sessionID.String()).
		Str("round_id", roundID.String()).
		Logger()

	log.Info().Msg("Starting FL round reward distribution")

	// Get session to check reward configuration
	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get FL session")
		return fmt.Errorf("failed to get FL session: %w", err)
	}

	// Get participants for this round
	participants, err := s.flParticipantRepo.GetByRound(ctx, roundID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get round participants")
		return fmt.Errorf("failed to get round participants: %w", err)
	}

	// Filter completed participants only
	completedParticipants := make([]*models.FLRoundParticipant, 0)
	for _, participant := range participants {
		if participant.Status == models.FLParticipantStatusCompleted {
			completedParticipants = append(completedParticipants, participant)
		}
	}

	if len(completedParticipants) == 0 {
		log.Info().Msg("No completed participants to reward")
		return nil
	}

	log.Info().
		Int("total_participants", len(participants)).
		Int("completed_participants", len(completedParticipants)).
		Msg("Found participants for reward distribution")

	// Calculate reward per participant based on their contribution
	totalRewardPool := s.calculateSessionRewardPool(session)
	roundRewardPool := totalRewardPool / float64(session.TotalRounds)

	log.Info().
		Float64("total_reward_pool", totalRewardPool).
		Float64("round_reward_pool", roundRewardPool).
		Msg("Calculated reward pools")

	// Distribute rewards to each completed participant
	for _, participant := range completedParticipants {
		participantReward := s.calculateParticipantReward(participant, roundRewardPool, completedParticipants)

		if participantReward <= 0 {
			log.Debug().
				Str("participant_id", participant.RunnerID).
				Float64("reward", participantReward).
				Msg("Skipping participant with zero reward")
			continue
		}

		if err := s.transferRewardToParticipant(ctx, session, participant, participantReward); err != nil {
			log.Error().Err(err).
				Str("participant_id", participant.RunnerID).
				Float64("reward", participantReward).
				Msg("Failed to transfer reward to participant")
			// Continue with other participants even if one fails
			continue
		}

		log.Info().
			Str("participant_id", participant.RunnerID).
			Float64("reward", participantReward).
			Msg("Successfully distributed reward to participant")
	}

	log.Info().Msg("FL round reward distribution completed")
	return nil
}

// DistributeSessionCompletionBonus distributes final bonus rewards when FL session completes
func (s *FLRewardService) DistributeSessionCompletionBonus(ctx context.Context, sessionID uuid.UUID) error {
	log := gologger.WithComponent("fl_rewards").With().
		Str("session_id", sessionID.String()).
		Logger()

	log.Info().Msg("Starting FL session completion bonus distribution")

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get FL session: %w", err)
	}

	// Calculate completion bonus (additional 10% of total reward pool)
	totalRewardPool := s.calculateSessionRewardPool(session)
	completionBonus := totalRewardPool * 0.1

	// Get all participants who participated in the session
	sessionParticipants, err := s.getSessionParticipants(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session participants: %w", err)
	}

	if len(sessionParticipants) == 0 {
		log.Info().Msg("No participants for completion bonus")
		return nil
	}

	// Distribute completion bonus equally among all participants
	bonusPerParticipant := completionBonus / float64(len(sessionParticipants))

	log.Info().
		Float64("completion_bonus", completionBonus).
		Float64("bonus_per_participant", bonusPerParticipant).
		Int("participant_count", len(sessionParticipants)).
		Msg("Calculated completion bonus")

	for _, participantID := range sessionParticipants {
		if err := s.transferCompletionBonus(ctx, session, participantID, bonusPerParticipant); err != nil {
			log.Error().Err(err).
				Str("participant_id", participantID).
				Float64("bonus", bonusPerParticipant).
				Msg("Failed to transfer completion bonus")
			continue
		}

		log.Info().
			Str("participant_id", participantID).
			Float64("bonus", bonusPerParticipant).
			Msg("Successfully distributed completion bonus")
	}

	log.Info().Msg("FL session completion bonus distribution completed")
	return nil
}

// Helper methods

func (s *FLRewardService) calculateSessionRewardPool(session *models.FederatedLearningSession) float64 {
	// Base reward calculation based on model complexity, rounds, and participants
	baseReward := 100.0 // Base USDFC amount

	// Scale by model complexity
	complexityMultiplier := 1.0
	if modelConfig, ok := session.Config.ModelConfig["model_type"].(string); ok {
		switch modelConfig {
		case "neural_network":
			complexityMultiplier = 2.0
		case "transformer":
			complexityMultiplier = 3.0
		case "linear_regression":
			complexityMultiplier = 1.0
		default:
			complexityMultiplier = 1.5
		}
	}

	// Scale by number of rounds and participants
	roundsMultiplier := float64(session.TotalRounds) * 0.1
	participantsMultiplier := float64(session.MinParticipants) * 0.5

	totalPool := baseReward * complexityMultiplier * (1 + roundsMultiplier) * (1 + participantsMultiplier)

	return totalPool
}

func (s *FLRewardService) calculateParticipantReward(
	participant *models.FLRoundParticipant,
	roundRewardPool float64,
	allParticipants []*models.FLRoundParticipant,
) float64 {
	// Base equal share
	baseReward := roundRewardPool / float64(len(allParticipants))

	// Performance bonus based on data size and training metrics
	performanceMultiplier := 1.0

	// Reward based on data contribution (more data = higher reward)
	if participant.DataSize > 0 {
		totalDataSize := 0
		for _, p := range allParticipants {
			totalDataSize += p.DataSize
		}
		if totalDataSize > 0 {
			dataContributionRatio := float64(participant.DataSize) / float64(totalDataSize)
			performanceMultiplier += dataContributionRatio * 0.5 // Up to 50% bonus for data contribution
		}
	}

	// Quality bonus based on training metrics (if available)
	if participant.TrainingMetrics != nil {
		// This would parse training metrics and add quality-based bonuses
		// For now, adding a small quality bonus
		performanceMultiplier += 0.1
	}

	finalReward := baseReward * performanceMultiplier
	return finalReward
}

func (s *FLRewardService) transferRewardToParticipant(
	ctx context.Context,
	session *models.FederatedLearningSession,
	participant *models.FLRoundParticipant,
	rewardAmount float64,
) error {
	log := gologger.WithComponent("fl_rewards").With().
		Str("session_id", session.ID.String()).
		Str("participant_id", participant.RunnerID).
		Float64("reward", rewardAmount).
		Logger()

	// Convert reward to wei (USDFC uses 18 decimals like ETH)
	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(rewardAmount),
		new(big.Float).SetFloat64(1e18),
	)
	rewardBigInt, _ := rewardWei.Int(nil)

	// Always use real wallet implementation - no more mock transfers
	return s.transferWithRealWallet(log, session.CreatorAddress, participant.RunnerID, rewardBigInt)
}

func (s *FLRewardService) transferCompletionBonus(
	ctx context.Context,
	session *models.FederatedLearningSession,
	participantID string,
	bonusAmount float64,
) error {
	// Convert bonus to wei
	bonusWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(bonusAmount),
		new(big.Float).SetFloat64(1e18),
	)
	bonusBigInt, _ := bonusWei.Int(nil)

	log := gologger.WithComponent("fl_rewards").With().
		Str("session_id", session.ID.String()).
		Str("participant_id", participantID).
		Float64("bonus", bonusAmount).
		Logger()

	// Always use real wallet implementation - no more mock transfers
	return s.transferWithRealWallet(log, session.CreatorAddress, participantID, bonusBigInt)
}

// Removed mock wallet transfer function - all transfers are now real blockchain transactions

func (s *FLRewardService) transferWithRealWallet(log zerolog.Logger, creatorAddress, participantID string, amount *big.Int) error {
	// Initialize real wallet client (same as docker reward distribution)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get home directory")
		return fmt.Errorf("home directory error: %w", err)
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, ".parity"),
		FileName: "keystore.json",
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create keystore")
		return fmt.Errorf("keystore creation failed: %w", err)
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Error().Err(err).Msg("Auth required")
		return fmt.Errorf("auth required: %w", err)
	}

	client, err := walletsdk.NewClient(walletsdk.ClientConfig{
		RPCURL:       s.cfg.FilecoinNetwork.RPC,
		ChainID:      int64(s.cfg.FilecoinNetwork.ChainID),
		TokenAddress: common.HexToAddress(s.cfg.FilecoinNetwork.TokenAddress),
		PrivateKey:   common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	})
	if err != nil {
		log.Error().Err(err).Msg("Client creation failed")
		return fmt.Errorf("wallet client failed: %w", err)
	}

	stakeWalletAddr := common.HexToAddress(s.cfg.FilecoinNetwork.StakeWalletAddress)
	stakeWallet, err := walletsdk.NewStakeWallet(
		client,
		stakeWalletAddr,
		common.HexToAddress(s.cfg.FilecoinNetwork.TokenAddress),
	)
	if err != nil {
		log.Error().Err(err).Msg("Stake wallet init failed")
		return fmt.Errorf("stake wallet init failed: %w", err)
	}

	// Check if participant has stake
	stakeInfo, err := stakeWallet.GetStakeInfo(participantID)
	if err != nil {
		log.Error().Err(err).Msg("Stake info check failed")
		return nil // Don't fail the entire process if one participant check fails
	}

	if !stakeInfo.Exists {
		log.Debug().Msg("No stake found for participant")
		return nil
	}

	log.Debug().
		Str("participant_stake", stakeInfo.Amount.String()).
		Str("transfer_amount", amount.String()).
		Msg("Initiating FL reward transfer")

	// Execute the transfer
	tx, err := stakeWallet.TransferPayment(creatorAddress, participantID, amount)
	if err != nil {
		log.Error().Err(err).Msg("Transfer failed")
		return fmt.Errorf("reward transfer failed: %w", err)
	}

	log.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Str("amount", amount.String()).
		Msg("FL reward transfer submitted")

	// Wait for confirmation
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().Err(err).
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Transfer confirmation failed")
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if receipt.Status == 0 {
		log.Error().
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Transfer reverted")
		return fmt.Errorf("transfer reverted")
	}

	log.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Str("amount", amount.String()).
		Str("block_number", receipt.BlockNumber.String()).
		Msg("FL reward transfer confirmed")

	return nil
}

func (s *FLRewardService) getSessionParticipants(ctx context.Context, sessionID uuid.UUID) ([]string, error) {
	// Get unique participants across all rounds of the session
	participants, err := s.flSessionRepo.GetParticipants(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session participants: %w", err)
	}

	// Remove duplicates
	uniqueParticipants := make(map[string]bool)
	for _, participant := range participants {
		uniqueParticipants[participant] = true
	}

	result := make([]string, 0, len(uniqueParticipants))
	for participant := range uniqueParticipants {
		result = append(result, participant)
	}

	return result, nil
}
