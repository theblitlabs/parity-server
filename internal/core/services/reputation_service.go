package services

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

const (
	// Reputation scoring for network quality control
	TASK_COMPLETED_SCORE       = 10
	TASK_FAILED_SCORE          = -15
	HIGH_QUALITY_BONUS         = 15
	POOR_QUALITY_PENALTY       = -20
	FAST_EXECUTION_BONUS       = 5
	SLOW_EXECUTION_PENALTY     = -5
	MALICIOUS_BEHAVIOR_PENALTY = -100
	CONSISTENCY_BONUS          = 10

	// Thresholds for network participation
	QUALITY_THRESHOLD = 500  // Minimum score for good standing
	BAN_THRESHOLD     = -100 // Score below which runner gets banned
	WARNING_THRESHOLD = 400  // Score below which runner gets warning

	// Ban criteria
	MAX_FAILURE_RATE      = 0.8 // 80% failure rate triggers investigation
	MAX_MALICIOUS_REPORTS = 3   // More than 3 reports triggers ban
	MIN_TASKS_FOR_BAN     = 5   // Minimum tasks before banning is possible
)

// Smart contract ABI for reputation contract
const reputationContractABI = `[
	{
		"inputs": [{"name": "runnerId", "type": "string"}, {"name": "walletAddress", "type": "address"}],
		"name": "registerRunner",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "runnerId", "type": "string"},
			{"name": "eventType", "type": "uint8"},
			{"name": "scoreDelta", "type": "int256"},
			{"name": "reason", "type": "string"}
		],
		"name": "updateReputation",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "runnerId", "type": "string"}, {"name": "reason", "type": "string"}],
		"name": "banRunner",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "runnerId", "type": "string"}],
		"name": "isRunnerEligible",
		"outputs": [{"name": "", "type": "bool"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "runnerId", "type": "string"}],
		"name": "isRunnerBanned",
		"outputs": [{"name": "", "type": "bool"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "runnerId", "type": "string"}],
		"name": "getRunnerStatus",
		"outputs": [
			{"name": "reputationScore", "type": "int256"},
			{"name": "status", "type": "uint8"},
			{"name": "totalTasks", "type": "uint256"},
			{"name": "successRate", "type": "uint256"},
			{"name": "isBanned", "type": "bool"}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

type ReputationService struct {
	reputationRepo  ports.ReputationRepository
	blockchainSvc   ports.ReputationBlockchainService
	ethClient       *ethclient.Client
	contractABI     abi.ABI
	contractAddress common.Address
}

func NewReputationService(
	reputationRepo ports.ReputationRepository,
	blockchainSvc ports.ReputationBlockchainService,
	ethRPCURL string,
	contractAddress string,
) (*ReputationService, error) {
	log := gologger.WithComponent("reputation_service")

	ethClient, err := ethclient.Dial(ethRPCURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to Ethereum client")
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(reputationContractABI))
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse contract ABI")
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	return &ReputationService{
		reputationRepo:  reputationRepo,
		blockchainSvc:   blockchainSvc,
		ethClient:       ethClient,
		contractABI:     contractABI,
		contractAddress: common.HexToAddress(contractAddress),
	}, nil
}

// RegisterRunner registers a new runner on the network
func (s *ReputationService) RegisterRunner(ctx context.Context, runnerID, walletAddress string) error {
	log := gologger.WithComponent("reputation_service")

	// Check if runner already exists
	if _, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID); err == nil {
		return fmt.Errorf("runner %s already registered", runnerID)
	}

	// Create initial reputation profile
	reputation := &models.RunnerReputation{
		ID:                     uuid.New(),
		RunnerID:               runnerID,
		WalletAddress:          walletAddress,
		ReputationScore:        500, // Starting score
		ReputationLevel:        models.ReputationLevelSilver,
		Status:                 models.ReputationStatusActive,
		TotalTasksCompleted:    0,
		TotalTasksFailed:       0,
		TaskSuccessRate:        0.0,
		AverageCompletionTime:  0.0,
		AverageQualityScore:    80.0, // Starting quality score
		ConsistencyScore:       80.0, // Starting consistency score
		UptimePercentage:       100.0,
		DockerExecutionScore:   50.0,
		LLMInferenceScore:      50.0,
		FederatedLearningScore: 50.0,
		StakeAmount:            0.0,
		TotalEarningsUSDFC:     0.0,
		LastActiveAt:           time.Now(),
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	// Save to database
	if err := s.reputationRepo.CreateRunnerReputation(ctx, reputation); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to create reputation record")
		return fmt.Errorf("failed to create reputation record: %w", err)
	}

	// Register on smart contract
	if err := s.registerRunnerOnContract(ctx, runnerID, walletAddress); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to register runner on smart contract")
		// Don't fail the registration if blockchain fails, but log the error
	}

	log.Info().Str("runner_id", runnerID).Str("wallet", walletAddress).Msg("Runner registered successfully")
	return nil
}

// IsRunnerEligible checks if a runner can participate in network tasks
func (s *ReputationService) IsRunnerEligible(ctx context.Context, runnerID string) (bool, error) {
	log := gologger.WithComponent("reputation_service")

	// First check smart contract (authoritative source)
	eligible, err := s.checkEligibilityOnContract(ctx, runnerID)
	if err != nil {
		log.Warn().Err(err).Str("runner_id", runnerID).Msg("Failed to check eligibility on contract, falling back to database")

		// Fallback to database check
		reputation, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID)
		if err != nil {
			return false, fmt.Errorf("runner not found: %w", err)
		}

		return reputation.Status == models.ReputationStatusActive ||
			reputation.Status == models.ReputationStatusWarning, nil
	}

	return eligible, nil
}

// IsRunnerBanned checks if a runner is banned from the network
func (s *ReputationService) IsRunnerBanned(ctx context.Context, runnerID string) (bool, error) {
	log := gologger.WithComponent("reputation_service")

	// Check smart contract first
	banned, err := s.checkBannedOnContract(ctx, runnerID)
	if err != nil {
		log.Warn().Err(err).Str("runner_id", runnerID).Msg("Failed to check ban status on contract")

		// Fallback to database
		reputation, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID)
		if err != nil {
			return false, fmt.Errorf("runner not found: %w", err)
		}

		return reputation.Status == models.ReputationStatusBanned, nil
	}

	return banned, nil
}

// UpdateReputationForTask updates reputation based on task completion
func (s *ReputationService) UpdateReputationForTask(ctx context.Context, runnerID string, taskResult *models.TaskResult) error {
	log := gologger.WithComponent("reputation_service")

	reputation, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("runner not found: %w", err)
	}

	// Calculate score changes based on task result
	var scoreDelta int
	var eventType models.ReputationEventType
	var reason string

	// Determine success based on exit code and error presence
	success := taskResult.ExitCode == 0 && taskResult.Error == ""

	if success {
		scoreDelta = TASK_COMPLETED_SCORE
		eventType = models.ReputationEventTypeTaskCompleted
		reason = "Task completed successfully"
		reputation.TotalTasksCompleted++

		// Quality bonuses based on execution metrics
		if taskResult.ExecutionTime < 30000 { // 30 seconds in milliseconds
			scoreDelta += FAST_EXECUTION_BONUS
			reason += " with fast execution"
		}
	} else {
		scoreDelta = TASK_FAILED_SCORE
		eventType = models.ReputationEventTypeTaskFailed
		reason = fmt.Sprintf("Task failed: %s", taskResult.Error)
		reputation.TotalTasksFailed++

		// Slow execution penalties
		if taskResult.ExecutionTime > 300000 { // 5 minutes in milliseconds
			scoreDelta += SLOW_EXECUTION_PENALTY
			reason += " and slow execution"
		}
	}

	// Calculate total tasks and success rate
	totalTasks := reputation.TotalTasksCompleted + reputation.TotalTasksFailed
	if totalTasks > 0 {
		reputation.TaskSuccessRate = float64(reputation.TotalTasksCompleted) / float64(totalTasks) * 100
	}

	// Update average response time
	if totalTasks > 0 {
		reputation.AverageCompletionTime = (reputation.AverageCompletionTime*float64(totalTasks-1) + float64(taskResult.ExecutionTime)) / float64(totalTasks)
	} else {
		reputation.AverageCompletionTime = float64(taskResult.ExecutionTime)
	}

	// Check for malicious behavior patterns
	if err := s.detectMaliciousBehavior(ctx, reputation, taskResult); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Error detecting malicious behavior")
	}

	// Apply reputation changes
	reputation.ReputationScore += scoreDelta
	reputation.UpdatedAt = time.Now()
	reputation.LastActiveAt = time.Now()

	// Check if runner should be banned
	shouldBan, banReason := s.shouldBanRunner(reputation)
	if shouldBan {
		reputation.Status = models.ReputationStatusBanned

		// Ban on smart contract
		if err := s.banRunnerOnContract(ctx, runnerID, banReason); err != nil {
			log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to ban runner on smart contract")
		}

		log.Warn().Str("runner_id", runnerID).Str("reason", banReason).Msg("Runner banned from network")
	} else if reputation.ReputationScore < WARNING_THRESHOLD {
		reputation.Status = models.ReputationStatusWarning
	} else if reputation.ReputationScore >= QUALITY_THRESHOLD {
		reputation.Status = models.ReputationStatusActive
	}

	// Save to database
	if err := s.reputationRepo.UpdateRunnerReputation(ctx, reputation); err != nil {
		return fmt.Errorf("failed to update reputation: %w", err)
	}

	// Log reputation event
	event := &models.ReputationEvent{
		ID:          uuid.New(),
		RunnerID:    runnerID,
		EventType:   eventType,
		ScoreDelta:  scoreDelta,
		NewScore:    reputation.ReputationScore,
		Description: reason,
		Metadata:    map[string]interface{}{"task_id": taskResult.TaskID.String()},
		CreatedAt:   time.Now(),
	}

	if err := s.reputationRepo.CreateReputationEvent(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to log reputation event")
	}

	// Update on smart contract
	if err := s.updateReputationOnContract(ctx, runnerID, eventType, scoreDelta, reason); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to update reputation on smart contract")
	}

	// Store snapshot on blockchain
	go func() {
		if err := s.storeReputationSnapshot(context.Background(), reputation); err != nil {
			log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to store reputation snapshot on blockchain")
		}
	}()

	log.Info().
		Str("runner_id", runnerID).
		Int("score_delta", scoreDelta).
		Int("new_score", reputation.ReputationScore).
		Str("status", string(reputation.Status)).
		Msg("Reputation updated for task completion")

	return nil
}

// ReportMaliciousBehavior reports malicious behavior for a runner
func (s *ReputationService) ReportMaliciousBehavior(ctx context.Context, runnerID, reason string, evidence map[string]interface{}) error {
	log := gologger.WithComponent("reputation_service")

	reputation, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("runner not found: %w", err)
	}

	// Apply heavy penalty for malicious behavior
	scoreDelta := MALICIOUS_BEHAVIOR_PENALTY
	reputation.ReputationScore += scoreDelta
	reputation.UpdatedAt = time.Now()

	// Immediately ban if this pushes them below ban threshold or if it's severe
	if reputation.ReputationScore <= BAN_THRESHOLD || strings.Contains(strings.ToLower(reason), "severe") {
		reputation.Status = models.ReputationStatusBanned

		// Ban on smart contract
		if err := s.banRunnerOnContract(ctx, runnerID, reason); err != nil {
			log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to ban malicious runner on smart contract")
		}

		log.Warn().Str("runner_id", runnerID).Str("reason", reason).Msg("Runner banned for malicious behavior")
	}

	// Save reputation update
	if err := s.reputationRepo.UpdateRunnerReputation(ctx, reputation); err != nil {
		return fmt.Errorf("failed to update reputation: %w", err)
	}

	// Log malicious behavior event
	event := &models.ReputationEvent{
		ID:          uuid.New(),
		RunnerID:    runnerID,
		EventType:   models.ReputationEventTypeMaliciousBehavior,
		ScoreDelta:  scoreDelta,
		NewScore:    reputation.ReputationScore,
		Description: reason,
		Metadata:    evidence,
		CreatedAt:   time.Now(),
	}

	if err := s.reputationRepo.CreateReputationEvent(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to log malicious behavior event")
	}

	// Update on smart contract
	if err := s.updateReputationOnContract(ctx, runnerID, models.ReputationEventTypeMaliciousBehavior, scoreDelta, reason); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to report malicious behavior on smart contract")
	}

	return nil
}

// Helper methods for smart contract interaction
func (s *ReputationService) registerRunnerOnContract(ctx context.Context, runnerID, walletAddress string) error {
	log := gologger.WithComponent("reputation_service")

	// Get private key from environment for contract interactions
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Warn().Msg("No private key available for contract interaction")
		return fmt.Errorf("no private key configured for blockchain operations")
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create authenticated transactor
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(314159)) // Filecoin Calibration Chain ID
	if err != nil {
		return fmt.Errorf("failed to create transactor: %w", err)
	}

	// Get the bound contract instance
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, s.ethClient, s.ethClient)

	// Call registerRunner function
	tx, err := contract.Transact(auth, "registerRunner", runnerID, common.HexToAddress(walletAddress))
	if err != nil {
		log.Error().Err(err).Msg("Failed to register runner on smart contract")
		return fmt.Errorf("failed to register runner on contract: %w", err)
	}

	log.Info().
		Str("runner_id", runnerID).
		Str("wallet", walletAddress).
		Str("tx_hash", tx.Hash().Hex()).
		Msg("Runner registered on smart contract")
	return nil
}

func (s *ReputationService) checkEligibilityOnContract(ctx context.Context, runnerID string) (bool, error) {
	// Create a contract caller
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, nil, nil)

	// Call the isRunnerEligible function
	var result []interface{}
	err := contract.Call(&bind.CallOpts{Context: ctx}, &result, "isRunnerEligible", runnerID)
	if err != nil {
		log := gologger.WithComponent("reputation_service")
		log.Error().Err(err).Msg("Failed to check eligibility on contract")
		return false, fmt.Errorf("failed to check contract eligibility: %w", err)
	}

	if len(result) > 0 {
		if eligible, ok := result[0].(bool); ok {
			return eligible, nil
		}
	}

	return false, fmt.Errorf("invalid response from contract")
}

func (s *ReputationService) checkBannedOnContract(ctx context.Context, runnerID string) (bool, error) {
	// Create a contract caller
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, nil, nil)

	// Call the isRunnerBanned function
	var result []interface{}
	err := contract.Call(&bind.CallOpts{Context: ctx}, &result, "isRunnerBanned", runnerID)
	if err != nil {
		log := gologger.WithComponent("reputation_service")
		log.Error().Err(err).Msg("Failed to check banned status on contract")
		return false, fmt.Errorf("failed to check contract banned status: %w", err)
	}

	if len(result) > 0 {
		if banned, ok := result[0].(bool); ok {
			return banned, nil
		}
	}

	return false, fmt.Errorf("invalid response from contract")
}

func (s *ReputationService) updateReputationOnContract(ctx context.Context, runnerID string, eventType models.ReputationEventType, scoreDelta int, reason string) error {
	log := gologger.WithComponent("reputation_service")

	// Get private key from environment for contract interactions
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Warn().Msg("No private key available for contract interaction")
		return fmt.Errorf("no private key configured for blockchain operations")
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create authenticated transactor
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(314159)) // Filecoin Calibration Chain ID
	if err != nil {
		return fmt.Errorf("failed to create transactor: %w", err)
	}

	// Get the bound contract instance
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, s.ethClient, s.ethClient)

	// Call updateReputation function with converted types
	bigScoreDelta := big.NewInt(int64(scoreDelta))
	tx, err := contract.Transact(auth, "updateReputation", runnerID, bigScoreDelta, reason)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update reputation on smart contract")
		return fmt.Errorf("failed to update reputation on contract: %w", err)
	}

	log.Info().
		Str("runner_id", runnerID).
		Str("event_type", string(eventType)).
		Int("score_delta", scoreDelta).
		Str("tx_hash", tx.Hash().Hex()).
		Msg("Reputation updated on smart contract")
	return nil
}

func (s *ReputationService) banRunnerOnContract(ctx context.Context, runnerID, reason string) error {
	log := gologger.WithComponent("reputation_service")

	// Get private key from environment for contract interactions
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Warn().Msg("No private key available for contract interaction")
		return fmt.Errorf("no private key configured for blockchain operations")
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create authenticated transactor
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(314159)) // Filecoin Calibration Chain ID
	if err != nil {
		return fmt.Errorf("failed to create transactor: %w", err)
	}

	// Get the bound contract instance
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, s.ethClient, s.ethClient)

	// Call banRunner function
	tx, err := contract.Transact(auth, "banRunner", runnerID, reason)
	if err != nil {
		log.Error().Err(err).Msg("Failed to ban runner on smart contract")
		return fmt.Errorf("failed to ban runner on contract: %w", err)
	}

	log.Info().
		Str("runner_id", runnerID).
		Str("reason", reason).
		Str("tx_hash", tx.Hash().Hex()).
		Msg("Runner banned on smart contract")
	return nil
}

// Helper methods for reputation logic
func (s *ReputationService) detectMaliciousBehavior(ctx context.Context, reputation *models.RunnerReputation, taskResult *models.TaskResult) error {
	log := gologger.WithComponent("reputation_service")

	// Pattern detection for malicious behavior
	totalTasks := reputation.TotalTasksCompleted + reputation.TotalTasksFailed

	// High failure rate
	if totalTasks >= MIN_TASKS_FOR_BAN && reputation.TaskSuccessRate < (1-MAX_FAILURE_RATE)*100 {
		return s.ReportMaliciousBehavior(ctx, reputation.RunnerID, "Consistently high failure rate indicating possible malicious behavior", map[string]interface{}{
			"failure_rate": 100 - reputation.TaskSuccessRate,
			"total_tasks":  totalTasks,
		})
	}

	// Suspicious task results (placeholder for more sophisticated detection)
	if taskResult != nil && taskResult.Error != "" && strings.Contains(strings.ToLower(taskResult.Error), "timeout") {
		// Could indicate resource hogging or deliberate slowdown
		log.Warn().Str("runner_id", reputation.RunnerID).Msg("Detected potential timeout-based malicious behavior")
	}

	return nil
}

func (s *ReputationService) shouldBanRunner(reputation *models.RunnerReputation) (bool, string) {
	// Check reputation score threshold
	if reputation.ReputationScore <= BAN_THRESHOLD {
		return true, "Reputation score below ban threshold"
	}

	// Check failure rate (must have minimum tasks)
	totalTasks := reputation.TotalTasksCompleted + reputation.TotalTasksFailed
	if totalTasks >= MIN_TASKS_FOR_BAN && reputation.TaskSuccessRate < (1-MAX_FAILURE_RATE)*100 {
		return true, fmt.Sprintf("High failure rate: %.1f%%", 100-reputation.TaskSuccessRate)
	}

	return false, ""
}

// Existing methods with network quality control focus
func (s *ReputationService) GetRunnerReputation(ctx context.Context, runnerID string) (*models.RunnerReputation, error) {
	log := gologger.WithComponent("reputation_service")

	reputation, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID)
	if err != nil {
		return nil, fmt.Errorf("reputation not found for runner %s: %w", runnerID, err)
	}

	// Check if runner is banned
	if reputation.Status == models.ReputationStatusBanned {
		log.Warn().Str("runner_id", runnerID).Msg("Attempted to get reputation for banned runner")
	}

	return reputation, nil
}

// SlashRunnerStake slashes a runner's stake for malicious behavior
func (s *ReputationService) SlashRunnerStake(ctx context.Context, runnerID string, reason string) error {
	log := gologger.WithComponent("reputation_service")

	log.Warn().
		Str("runner_id", runnerID).
		Str("reason", reason).
		Msg("Slashing runner stake for malicious behavior")

	// Get current reputation
	reputation, err := s.reputationRepo.GetRunnerReputation(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to get runner reputation: %w", err)
	}

	// Apply slashing penalty to reputation
	oldScore := reputation.ReputationScore
	reputation.ReputationScore -= 100 // Heavy penalty for slashing
	if reputation.ReputationScore < 0 {
		reputation.ReputationScore = 0
	}

	// Update status based on new score
	if reputation.ReputationScore <= BAN_THRESHOLD {
		reputation.Status = models.ReputationStatusBanned
		// Ban on smart contract as well
		if err := s.banRunnerOnContract(ctx, runnerID, fmt.Sprintf("Stake slashed: %s", reason)); err != nil {
			log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to ban runner on contract after slashing")
		}
	} else if reputation.ReputationScore <= WARNING_THRESHOLD {
		reputation.Status = models.ReputationStatusWarning
	}

	reputation.UpdatedAt = time.Now()

	// Create slashing event
	event := &models.ReputationEvent{
		ID:          uuid.New(),
		RunnerID:    runnerID,
		EventType:   models.ReputationEventTypeSlashing,
		ScoreDelta:  -(oldScore - reputation.ReputationScore),
		NewScore:    reputation.ReputationScore,
		Description: fmt.Sprintf("Stake slashed: %s", reason),
		CreatedAt:   time.Now(),
	}

	// Save updates
	if err := s.reputationRepo.UpdateRunnerReputation(ctx, reputation); err != nil {
		return fmt.Errorf("failed to update reputation after slashing: %w", err)
	}

	if err := s.reputationRepo.CreateReputationEvent(ctx, event); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to save slashing event")
	}

	// Update on smart contract
	if err := s.updateReputationOnContract(ctx, runnerID, models.ReputationEventTypeSlashing, -100, reason); err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to update reputation on contract after slashing")
	}

	log.Info().
		Str("runner_id", runnerID).
		Int("old_score", oldScore).
		Int("new_score", reputation.ReputationScore).
		Str("reason", reason).
		Msg("Runner stake slashed successfully")

	return nil
}

func (s *ReputationService) GetLeaderboard(ctx context.Context, leaderboardType string, limit int) ([]*models.RunnerReputation, error) {
	var reputations []*models.RunnerReputation
	var err error

	switch leaderboardType {
	case "overall":
		reputations, err = s.reputationRepo.GetTopRunners(ctx, limit)
	default:
		return nil, fmt.Errorf("unknown leaderboard type: %s", leaderboardType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	// Filter out banned runners from public leaderboards
	activeReputations := make([]*models.RunnerReputation, 0, len(reputations))
	for _, rep := range reputations {
		if rep.Status != models.ReputationStatusBanned {
			activeReputations = append(activeReputations, rep)
		}
	}

	return activeReputations, nil
}

func (s *ReputationService) GetNetworkStats(ctx context.Context) (*models.NetworkStats, error) {
	stats, err := s.reputationRepo.GetReputationStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}

	// Convert interface{} stats to NetworkStats structure
	networkStats := &models.NetworkStats{}

	if totalRunners, ok := stats["total_runners"].(int64); ok {
		networkStats.TotalRunners = int(totalRunners)
	}
	if activeRunners, ok := stats["active_runners"].(int64); ok {
		networkStats.ActiveRunners = int(activeRunners)
	}
	if bannedRunners, ok := stats["banned_runners"].(int64); ok {
		networkStats.BannedRunners = int(bannedRunners)
	}

	// Add network health indicators
	if networkStats.TotalRunners > 0 {
		bannedRate := float64(networkStats.BannedRunners) / float64(networkStats.TotalRunners) * 100
		networkStats.NetworkHealth = "healthy"

		if bannedRate > 20 {
			networkStats.NetworkHealth = "degraded"
		}
		if bannedRate > 50 {
			networkStats.NetworkHealth = "critical"
		}
	}

	return networkStats, nil
}

// storeReputationSnapshot stores a reputation snapshot via blockchain service
func (s *ReputationService) storeReputationSnapshot(ctx context.Context, reputation *models.RunnerReputation) error {
	if s.blockchainSvc == nil {
		return fmt.Errorf("blockchain service not available")
	}

	// Store reputation data on IPFS and get hash
	ipfsHash, err := s.blockchainSvc.StoreReputationData(ctx, reputation)
	if err != nil {
		return fmt.Errorf("failed to store reputation data: %w", err)
	}

	// Register the hash on-chain
	_, err = s.blockchainSvc.RegisterReputationHash(ctx, reputation.RunnerID, ipfsHash)
	if err != nil {
		return fmt.Errorf("failed to register reputation hash: %w", err)
	}

	return nil
}
