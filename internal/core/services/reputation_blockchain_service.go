package services

import (
	"context"
	"encoding/json"
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
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/config"
)

// Embedded ABI for RunnerReputation contract
const runnerReputationABI = `[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"BAN_DURATION","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"BAN_THRESHOLD","inputs":[],"outputs":[{"name":"","type":"int256","internalType":"int256"}],"stateMutability":"view"},{"type":"function","name":"MIN_STAKE_AMOUNT","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"MONITORING_DURATION","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"MONITORING_REWARD","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"QUALITY_THRESHOLD","inputs":[],"outputs":[{"name":"","type":"int256","internalType":"int256"}],"stateMutability":"view"},{"type":"function","name":"SLASHING_PERCENTAGE","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"STARTING_SCORE","inputs":[],"outputs":[{"name":"","type":"int256","internalType":"int256"}],"stateMutability":"view"},{"type":"function","name":"activeMonitoringIds","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"type":"function","name":"assignRandomMonitoring","inputs":[],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"authorizedReporters","inputs":[{"name":"","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"type":"function","name":"banRunner","inputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"reason","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"depositStake","inputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"amount","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"getActiveMonitoringAssignments","inputs":[],"outputs":[{"name":"","type":"string[]","internalType":"string[]"}],"stateMutability":"view"},{"type":"function","name":"getMonitoringAssignment","inputs":[{"name":"assignmentId","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"tuple","internalType":"struct RunnerReputation.MonitoringAssignment","components":[{"name":"monitorId","type":"string","internalType":"string"},{"name":"targetId","type":"string","internalType":"string"},{"name":"startTime","type":"uint256","internalType":"uint256"},{"name":"duration","type":"uint256","internalType":"uint256"},{"name":"isActive","type":"bool","internalType":"bool"},{"name":"reportSubmitted","type":"bool","internalType":"bool"},{"name":"reportType","type":"uint8","internalType":"enum RunnerReputation.MonitoringReportType"},{"name":"evidenceHash","type":"string","internalType":"string"},{"name":"submissionTime","type":"uint256","internalType":"uint256"},{"name":"verified","type":"bool","internalType":"bool"},{"name":"monitorReward","type":"int256","internalType":"int256"}]}],"stateMutability":"view"},{"type":"function","name":"getNetworkStats","inputs":[],"outputs":[{"name":"","type":"tuple","internalType":"struct RunnerReputation.NetworkStats","components":[{"name":"totalRunners","type":"uint256","internalType":"uint256"},{"name":"activeRunners","type":"uint256","internalType":"uint256"},{"name":"bannedRunners","type":"uint256","internalType":"uint256"},{"name":"averageReputation","type":"int256","internalType":"int256"},{"name":"totalTasks","type":"uint256","internalType":"uint256"},{"name":"activeMonitoringAssignments","type":"uint256","internalType":"uint256"},{"name":"totalSlashedAmount","type":"uint256","internalType":"uint256"},{"name":"totalStakedAmount","type":"uint256","internalType":"uint256"}]}],"stateMutability":"view"},{"type":"function","name":"getReputationEvents","inputs":[{"name":"runnerId","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"tuple[]","internalType":"struct RunnerReputation.ReputationEvent[]","components":[{"name":"eventType","type":"uint8","internalType":"enum RunnerReputation.ReputationEventType"},{"name":"scoreDelta","type":"int256","internalType":"int256"},{"name":"timestamp","type":"uint256","internalType":"uint256"},{"name":"reason","type":"string","internalType":"string"},{"name":"reporter","type":"address","internalType":"address"},{"name":"relatedMonitoringId","type":"string","internalType":"string"}]}],"stateMutability":"view"},{"type":"function","name":"getRunnerMonitoringHistory","inputs":[{"name":"runnerId","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"string[]","internalType":"string[]"}],"stateMutability":"view"},{"type":"function","name":"getRunnerProfile","inputs":[{"name":"runnerId","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"tuple","internalType":"struct RunnerReputation.RunnerProfile","components":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"walletAddress","type":"address","internalType":"address"},{"name":"reputationScore","type":"int256","internalType":"int256"},{"name":"status","type":"uint8","internalType":"enum RunnerReputation.RunnerStatus"},{"name":"totalTasks","type":"uint256","internalType":"uint256"},{"name":"successfulTasks","type":"uint256","internalType":"uint256"},{"name":"failedTasks","type":"uint256","internalType":"uint256"},{"name":"maliciousReports","type":"uint256","internalType":"uint256"},{"name":"joinedAt","type":"uint256","internalType":"uint256"},{"name":"lastActiveAt","type":"uint256","internalType":"uint256"},{"name":"bannedAt","type":"uint256","internalType":"uint256"},{"name":"banReason","type":"string","internalType":"string"},{"name":"ipfsDataHash","type":"string","internalType":"string"},{"name":"stakedAmount","type":"uint256","internalType":"uint256"},{"name":"totalEarnings","type":"uint256","internalType":"uint256"},{"name":"totalSlashed","type":"uint256","internalType":"uint256"},{"name":"monitoringScore","type":"uint256","internalType":"uint256"},{"name":"timesMonitored","type":"uint256","internalType":"uint256"},{"name":"timesMonitoring","type":"uint256","internalType":"uint256"}]}],"stateMutability":"view"},{"type":"function","name":"getRunnerStatus","inputs":[{"name":"runnerId","type":"string","internalType":"string"}],"outputs":[{"name":"reputationScore","type":"int256","internalType":"int256"},{"name":"status","type":"uint8","internalType":"enum RunnerReputation.RunnerStatus"},{"name":"totalTasks","type":"uint256","internalType":"uint256"},{"name":"successRate","type":"uint256","internalType":"uint256"},{"name":"isBanned","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"type":"function","name":"getRunners","inputs":[{"name":"offset","type":"uint256","internalType":"uint256"},{"name":"limit","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string[]","internalType":"string[]"}],"stateMutability":"view"},{"type":"function","name":"getTopRunners","inputs":[{"name":"limit","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"topRunners","type":"string[]","internalType":"string[]"},{"name":"scores","type":"int256[]","internalType":"int256[]"}],"stateMutability":"view"},{"type":"function","name":"isRunnerBanned","inputs":[{"name":"runnerId","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"type":"function","name":"isRunnerEligible","inputs":[{"name":"runnerId","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"type":"function","name":"monitoringAssignments","inputs":[{"name":"","type":"string","internalType":"string"}],"outputs":[{"name":"monitorId","type":"string","internalType":"string"},{"name":"targetId","type":"string","internalType":"string"},{"name":"startTime","type":"uint256","internalType":"uint256"},{"name":"duration","type":"uint256","internalType":"uint256"},{"name":"isActive","type":"bool","internalType":"bool"},{"name":"reportSubmitted","type":"bool","internalType":"bool"},{"name":"reportType","type":"uint8","internalType":"enum RunnerReputation.MonitoringReportType"},{"name":"evidenceHash","type":"string","internalType":"string"},{"name":"submissionTime","type":"uint256","internalType":"uint256"},{"name":"verified","type":"bool","internalType":"bool"},{"name":"monitorReward","type":"int256","internalType":"int256"}],"stateMutability":"view"},{"type":"function","name":"networkStats","inputs":[],"outputs":[{"name":"totalRunners","type":"uint256","internalType":"uint256"},{"name":"activeRunners","type":"uint256","internalType":"uint256"},{"name":"bannedRunners","type":"uint256","internalType":"uint256"},{"name":"averageReputation","type":"int256","internalType":"int256"},{"name":"totalTasks","type":"uint256","internalType":"uint256"},{"name":"activeMonitoringAssignments","type":"uint256","internalType":"uint256"},{"name":"totalSlashedAmount","type":"uint256","internalType":"uint256"},{"name":"totalStakedAmount","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"nextMonitoringId","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"owner","inputs":[],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"type":"function","name":"permanentBans","inputs":[{"name":"","type":"string","internalType":"string"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"type":"function","name":"registerRunner","inputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"walletAddress","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"reputationEvents","inputs":[{"name":"","type":"string","internalType":"string"},{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"eventType","type":"uint8","internalType":"enum RunnerReputation.ReputationEventType"},{"name":"scoreDelta","type":"int256","internalType":"int256"},{"name":"timestamp","type":"uint256","internalType":"uint256"},{"name":"reason","type":"string","internalType":"string"},{"name":"reporter","type":"address","internalType":"address"},{"name":"relatedMonitoringId","type":"string","internalType":"string"}],"stateMutability":"view"},{"type":"function","name":"runnerIds","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"type":"function","name":"runnerMonitoringHistory","inputs":[{"name":"","type":"string","internalType":"string"},{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"type":"function","name":"runners","inputs":[{"name":"","type":"string","internalType":"string"}],"outputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"walletAddress","type":"address","internalType":"address"},{"name":"reputationScore","type":"int256","internalType":"int256"},{"name":"status","type":"uint8","internalType":"enum RunnerReputation.RunnerStatus"},{"name":"totalTasks","type":"uint256","internalType":"uint256"},{"name":"successfulTasks","type":"uint256","internalType":"uint256"},{"name":"failedTasks","type":"uint256","internalType":"uint256"},{"name":"maliciousReports","type":"uint256","internalType":"uint256"},{"name":"joinedAt","type":"uint256","internalType":"uint256"},{"name":"lastActiveAt","type":"uint256","internalType":"uint256"},{"name":"bannedAt","type":"uint256","internalType":"uint256"},{"name":"banReason","type":"string","internalType":"string"},{"name":"ipfsDataHash","type":"string","internalType":"string"},{"name":"stakedAmount","type":"uint256","internalType":"uint256"},{"name":"totalEarnings","type":"uint256","internalType":"uint256"},{"name":"totalSlashed","type":"uint256","internalType":"uint256"},{"name":"monitoringScore","type":"uint256","internalType":"uint256"},{"name":"timesMonitored","type":"uint256","internalType":"uint256"},{"name":"timesMonitoring","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"type":"function","name":"slashStake","inputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"reason","type":"string","internalType":"string"}],"outputs":[{"name":"slashedAmount","type":"uint256","internalType":"uint256"}],"stateMutability":"nonpayable"},{"type":"function","name":"submitMonitoringReport","inputs":[{"name":"assignmentId","type":"string","internalType":"string"},{"name":"reportType","type":"uint8","internalType":"enum RunnerReputation.MonitoringReportType"},{"name":"evidenceHash","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"updateReputation","inputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"eventType","type":"uint8","internalType":"enum RunnerReputation.ReputationEventType"},{"name":"scoreDelta","type":"int256","internalType":"int256"},{"name":"reason","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"walletToRunnerId","inputs":[{"name":"","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"type":"function","name":"withdrawStake","inputs":[{"name":"runnerId","type":"string","internalType":"string"},{"name":"amount","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"event","name":"BanThresholdUpdated","inputs":[{"name":"oldThreshold","type":"int256","indexed":false,"internalType":"int256"},{"name":"newThreshold","type":"int256","indexed":false,"internalType":"int256"}],"anonymous":false},{"type":"event","name":"IPFSHashUpdated","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"ipfsHash","type":"string","indexed":false,"internalType":"string"}],"anonymous":false},{"type":"event","name":"MonitoringAssigned","inputs":[{"name":"monitorId","type":"string","indexed":true,"internalType":"string"},{"name":"targetId","type":"string","indexed":true,"internalType":"string"},{"name":"duration","type":"uint256","indexed":false,"internalType":"uint256"}],"anonymous":false},{"type":"event","name":"MonitoringReportSubmitted","inputs":[{"name":"monitorId","type":"string","indexed":true,"internalType":"string"},{"name":"targetId","type":"string","indexed":true,"internalType":"string"},{"name":"reportType","type":"uint8","indexed":false,"internalType":"uint8"},{"name":"evidence","type":"string","indexed":false,"internalType":"string"}],"anonymous":false},{"type":"event","name":"QualityThresholdUpdated","inputs":[{"name":"oldThreshold","type":"int256","indexed":false,"internalType":"int256"},{"name":"newThreshold","type":"int256","indexed":false,"internalType":"int256"}],"anonymous":false},{"type":"event","name":"ReputationUpdated","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"oldScore","type":"int256","indexed":false,"internalType":"int256"},{"name":"newScore","type":"int256","indexed":false,"internalType":"int256"},{"name":"reason","type":"string","indexed":false,"internalType":"string"}],"anonymous":false},{"type":"event","name":"RunnerBanned","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"walletAddress","type":"address","indexed":true,"internalType":"address"},{"name":"reason","type":"string","indexed":false,"internalType":"string"}],"anonymous":false},{"type":"event","name":"RunnerRegistered","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"walletAddress","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"type":"event","name":"RunnerUnbanned","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"walletAddress","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"type":"event","name":"SlashingExecuted","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"wallet","type":"address","indexed":true,"internalType":"address"},{"name":"amount","type":"uint256","indexed":false,"internalType":"uint256"},{"name":"reason","type":"string","indexed":false,"internalType":"string"}],"anonymous":false},{"type":"event","name":"StakeDeposited","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"wallet","type":"address","indexed":true,"internalType":"address"},{"name":"amount","type":"uint256","indexed":false,"internalType":"uint256"}],"anonymous":false},{"type":"event","name":"StakeWithdrawn","inputs":[{"name":"runnerId","type":"string","indexed":true,"internalType":"string"},{"name":"wallet","type":"address","indexed":true,"internalType":"address"},{"name":"amount","type":"uint256","indexed":false,"internalType":"uint256"}],"anonymous":false}]`

type ReputationBlockchainService struct {
	ethClient         *ethclient.Client
	contractABI       abi.ABI
	contractAddress   common.Address
	blockchainService *BlockchainService
	config            *config.Config
	privateKey        string
}

func NewReputationBlockchainService(
	cfg *config.Config,
	blockchainService *BlockchainService,
) (*ReputationBlockchainService, error) {
	log := gologger.WithComponent("reputation_blockchain")

	// Connect to blockchain client
	ethClient, err := ethclient.Dial(cfg.BlockchainNetwork.RPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blockchain client: %w", err)
	}

	// Load contract ABI (now uses embedded ABI)
	contractABI, err := loadContractABI()
	if err != nil {
		return nil, fmt.Errorf("failed to load contract ABI: %w", err)
	}

	// Parse contract address
	contractAddress := common.HexToAddress(cfg.SmartContract.ReputationContractAddress)

	// Get private key from environment
	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		log.Warn().Msg("No PRIVATE_KEY found in environment, blockchain write operations will be limited")
	}

	service := &ReputationBlockchainService{
		ethClient:         ethClient,
		contractABI:       contractABI,
		contractAddress:   contractAddress,
		blockchainService: blockchainService,
		config:            cfg,
		privateKey:        privateKey,
	}

	log.Info().
		Str("contract_address", service.contractAddress.Hex()).
		Str("network", cfg.BlockchainNetwork.NetworkName).
		Msg("Reputation blockchain service initialized")

	return service, nil
}

// StoreReputationData stores reputation data to IPFS and returns the hash
func (s *ReputationBlockchainService) StoreReputationData(ctx context.Context, data interface{}) (string, error) {
	log := gologger.WithComponent("reputation_ipfs")

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal data")
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	// Use the actual BlockchainService to store data
	cid, err := s.blockchainService.ipfsClient.Add(strings.NewReader(string(jsonData)))
	if err != nil {
		log.Error().Err(err).Msg("Failed to store data to IPFS")
		return "", fmt.Errorf("failed to store data to IPFS: %w", err)
	}

	log.Info().
		Str("ipfs_hash", cid).
		Int("data_size", len(jsonData)).
		Msg("Reputation data stored to IPFS")

	return cid, nil
}

// RetrieveReputationData retrieves reputation data from IPFS
func (s *ReputationBlockchainService) RetrieveReputationData(ctx context.Context, ipfsHash string) (interface{}, error) {
	log := gologger.WithComponent("reputation_ipfs")

	// Retrieve data from IPFS using the BlockchainService
	reader, err := s.blockchainService.ipfsClient.Cat(ipfsHash)
	if err != nil {
		log.Error().Err(err).Str("ipfs_hash", ipfsHash).Msg("Failed to retrieve data from IPFS")
		return nil, fmt.Errorf("failed to retrieve data from IPFS: %w", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close IPFS reader")
		}
	}()

	var reputationData interface{}
	if err := json.NewDecoder(reader).Decode(&reputationData); err != nil {
		log.Error().Err(err).Str("ipfs_hash", ipfsHash).Msg("Failed to decode retrieved data")
		return nil, fmt.Errorf("failed to decode retrieved data: %w", err)
	}

	log.Info().
		Str("ipfs_hash", ipfsHash).
		Msg("Reputation data retrieved from IPFS")

	return reputationData, nil
}

// CreateBlockchainDeal creates a storage deal on blockchain network
func (s *ReputationBlockchainService) CreateBlockchainDeal(ctx context.Context, ipfsHash string) (string, error) {
	log := gologger.WithComponent("reputation_blockchain")

	// For now, this returns the IPFS hash as the deal ID since actual blockchain deal creation
	// requires more complex integration with blockchain storage providers
	// The data is already stored and accessible via IPFS
	dealID := fmt.Sprintf("bc_%s", ipfsHash)

	log.Info().
		Str("ipfs_hash", ipfsHash).
		Str("deal_id", dealID).
		Msg("Blockchain storage deal marked for IPFS hash")

	return dealID, nil
}

// VerifyBlockchainDeal verifies that data is accessible via IPFS
func (s *ReputationBlockchainService) VerifyBlockchainDeal(ctx context.Context, dealID string) (bool, error) {
	log := gologger.WithComponent("reputation_blockchain")

	// Extract IPFS hash from deal ID
	ipfsHash := strings.TrimPrefix(dealID, "bc_")

	// Verify by attempting to retrieve the data
	_, err := s.blockchainService.ipfsClient.Cat(ipfsHash)
	if err != nil {
		log.Error().Err(err).Str("deal_id", dealID).Msg("Failed to verify blockchain deal")
		return false, nil
	}

	log.Info().
		Str("deal_id", dealID).
		Msg("Blockchain storage deal verified")

	return true, nil
}

// RegisterReputationHash registers the IPFS hash on-chain via smart contract
func (s *ReputationBlockchainService) RegisterReputationHash(ctx context.Context, runnerID, ipfsHash string) (string, error) {
	log := gologger.WithComponent("reputation_onchain").With().
		Str("runner_id", runnerID).
		Str("ipfs_hash", ipfsHash).
		Logger()

	if s.privateKey == "" {
		log.Warn().Msg("No private key available, cannot submit on-chain transaction")
		return "", fmt.Errorf("no private key configured for blockchain operations")
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create authenticated transactor
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(s.config.BlockchainNetwork.ChainID)))
	if err != nil {
		return "", fmt.Errorf("failed to create transactor: %w", err)
	}

	// Get the bound contract instance
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, s.ethClient, s.ethClient)

	// Call updateReputationMetadata function
	tx, err := contract.Transact(auth, "updateReputationMetadata", runnerID, ipfsHash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to submit reputation hash to blockchain")
		return "", fmt.Errorf("failed to submit transaction: %w", err)
	}

	log.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Msg("Reputation hash registered on-chain")

	return tx.Hash().Hex(), nil
}

// VerifyOnChainHash verifies if a reputation hash is registered on-chain
func (s *ReputationBlockchainService) VerifyOnChainHash(ctx context.Context, runnerID, ipfsHash string) (bool, error) {
	log := gologger.WithComponent("reputation_verify").With().
		Str("runner_id", runnerID).
		Str("ipfs_hash", ipfsHash).
		Logger()

	// Create a contract caller
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, nil, nil)

	// Call the getRunnerProfile function to get metadata
	var result []interface{}
	err := contract.Call(&bind.CallOpts{Context: ctx}, &result, "getRunnerProfile", runnerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query runner profile from contract")
		return false, fmt.Errorf("failed to query contract: %w", err)
	}

	// The getRunnerProfile returns a struct with metadata field
	if len(result) > 0 {
		// Extract metadata from the profile struct (assuming it's at index 6 based on contract)
		if profile, ok := result[0].(struct {
			Id              string
			ReputationScore *big.Int
			TasksCompleted  *big.Int
			TasksFailed     *big.Int
			LastActiveTime  *big.Int
			Status          uint8
			Metadata        string
			StakedAmount    *big.Int
			TotalSlashed    *big.Int
			TotalEarnings   *big.Int
			TimesMonitoring *big.Int
			TimesMonitored  *big.Int
			MonitoringScore *big.Int
		}); ok {
			verified := profile.Metadata == ipfsHash
			log.Info().Bool("verified", verified).Msg("Reputation hash verification completed")
			return verified, nil
		}
	}

	log.Info().Bool("verified", false).Msg("Reputation hash not found on-chain")
	return false, nil
}

// GetPublicReputationData returns publicly verifiable reputation data from blockchain
func (s *ReputationBlockchainService) GetPublicReputationData(ctx context.Context, runnerID string) (map[string]interface{}, error) {
	log := gologger.WithComponent("reputation_public").With().
		Str("runner_id", runnerID).
		Logger()

	// Query the smart contract for runner profile
	contract := bind.NewBoundContract(s.contractAddress, s.contractABI, s.ethClient, nil, nil)

	var result []interface{}
	err := contract.Call(&bind.CallOpts{Context: ctx}, &result, "getRunnerProfile", runnerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get runner profile from blockchain")
		return nil, fmt.Errorf("failed to query blockchain: %w", err)
	}

	if len(result) == 0 {
		return map[string]interface{}{
			"runner_id": runnerID,
			"error":     "Runner not found on blockchain",
		}, nil
	}

	// Extract data from the contract response
	publicData := map[string]interface{}{
		"runner_id":        runnerID,
		"contract_address": s.contractAddress.Hex(),
		"network":          s.config.BlockchainNetwork.NetworkName,
		"last_updated":     time.Now().Format(time.RFC3339),
		"verification":     "on-chain",
		"ipfs_gateway":     s.config.Blockchain.GatewayURL,
	}

	// Add contract data if available
	if profile, ok := result[0].(struct {
		Id              string
		ReputationScore *big.Int
		TasksCompleted  *big.Int
		TasksFailed     *big.Int
		LastActiveTime  *big.Int
		Status          uint8
		Metadata        string
		StakedAmount    *big.Int
		TotalSlashed    *big.Int
		TotalEarnings   *big.Int
		TimesMonitoring *big.Int
		TimesMonitored  *big.Int
		MonitoringScore *big.Int
	}); ok {
		publicData["reputation_score"] = profile.ReputationScore.Int64()
		publicData["tasks_completed"] = profile.TasksCompleted.Int64()
		publicData["tasks_failed"] = profile.TasksFailed.Int64()
		publicData["staked_amount"] = profile.StakedAmount.String()
		publicData["metadata_hash"] = profile.Metadata
	}

	log.Info().Msg("Retrieved public reputation data from blockchain")
	return publicData, nil
}

// VerifyReputationIntegrity performs comprehensive integrity verification
func (s *ReputationBlockchainService) VerifyReputationIntegrity(ctx context.Context, runnerID string) (bool, error) {
	log := gologger.WithComponent("reputation_integrity").With().
		Str("runner_id", runnerID).
		Logger()

	log.Info().Msg("Performing reputation integrity verification")

	// Step 1: Get on-chain profile
	publicData, err := s.GetPublicReputationData(ctx, runnerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get on-chain reputation data")
		return false, err
	}

	if errorMsg, hasError := publicData["error"]; hasError {
		log.Warn().Str("error", errorMsg.(string)).Msg("Runner not found on blockchain")
		return false, nil
	}

	// Step 2: Verify IPFS metadata if present
	if metadataHash, exists := publicData["metadata_hash"]; exists && metadataHash != "" {
		hashStr := metadataHash.(string)
		if hashStr != "" {
			// Verify IPFS data accessibility
			_, err := s.RetrieveReputationData(ctx, hashStr)
			if err != nil {
				log.Error().Err(err).Str("metadata_hash", hashStr).Msg("Failed to retrieve IPFS metadata")
				return false, nil
			}

			// Verify on-chain hash matches IPFS hash
			verified, err := s.VerifyOnChainHash(ctx, runnerID, hashStr)
			if err != nil {
				log.Error().Err(err).Msg("Failed to verify on-chain hash")
				return false, err
			}

			if !verified {
				log.Warn().Msg("On-chain hash verification failed")
				return false, nil
			}
		}
	}

	// Step 3: Verify contract is accessible and responding
	_, err = s.ethClient.ChainID(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to verify blockchain connectivity")
		return false, err
	}

	log.Info().Msg("Reputation integrity verification completed successfully")
	return true, nil
}

// Helper function to load contract ABI (now uses embedded ABI)
func loadContractABI() (abi.ABI, error) {
	// Use embedded ABI instead of reading from file
	contractABI, err := abi.JSON(strings.NewReader(runnerReputationABI))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to parse embedded contract ABI: %w", err)
	}

	return contractABI, nil
}
