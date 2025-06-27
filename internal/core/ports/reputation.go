package ports

import (
	"context"

	"github.com/theblitlabs/parity-server/internal/core/models"
)

// ReputationRepository defines the interface for reputation persistence
type ReputationRepository interface {
	// Runner Reputation Management
	CreateRunnerReputation(ctx context.Context, reputation *models.RunnerReputation) error
	GetRunnerReputation(ctx context.Context, runnerID string) (*models.RunnerReputation, error)
	UpdateRunnerReputation(ctx context.Context, reputation *models.RunnerReputation) error
	GetTopRunners(ctx context.Context, limit int) ([]*models.RunnerReputation, error)
	GetRunnersByLevel(ctx context.Context, level string) ([]*models.RunnerReputation, error)

	// Reputation Events
	CreateReputationEvent(ctx context.Context, event *models.ReputationEvent) error
	GetRunnerEvents(ctx context.Context, runnerID string, limit int) ([]*models.ReputationEvent, error)
	GetPublicEvents(ctx context.Context, limit int) ([]*models.ReputationEvent, error)
	GetEventsByType(ctx context.Context, eventType string, limit int) ([]*models.ReputationEvent, error)

	// Reputation Snapshots
	CreateReputationSnapshot(ctx context.Context, snapshot *models.ReputationSnapshot) error
	GetLatestSnapshot(ctx context.Context, runnerID string) (*models.ReputationSnapshot, error)
	GetSnapshotHistory(ctx context.Context, runnerID string, limit int) ([]*models.ReputationSnapshot, error)
	GetSnapshotsByType(ctx context.Context, snapshotType string, limit int) ([]*models.ReputationSnapshot, error)

	// Leaderboards
	CreateLeaderboard(ctx context.Context, leaderboard *models.ReputationLeaderboard) error
	GetLeaderboard(ctx context.Context, leaderboardType, period string) (*models.ReputationLeaderboard, error)
	UpdateLeaderboard(ctx context.Context, leaderboard *models.ReputationLeaderboard) error
	GetAllLeaderboards(ctx context.Context) ([]*models.ReputationLeaderboard, error)

	// Analytics
	GetReputationStats(ctx context.Context) (map[string]interface{}, error)
	GetRunnerRankings(ctx context.Context, specialty string) ([]*models.RunnerReputation, error)
	SearchRunners(ctx context.Context, query string, filters map[string]interface{}) ([]*models.RunnerReputation, error)
}

// ReputationBlockchainService defines blockchain integration for reputation
type ReputationBlockchainService interface {
	// IPFS Storage
	StoreReputationData(ctx context.Context, data interface{}) (string, error) // Returns IPFS hash
	RetrieveReputationData(ctx context.Context, ipfsHash string) (interface{}, error)

	// Filecoin Storage
	CreateFilecoinDeal(ctx context.Context, ipfsHash string) (string, error) // Returns deal ID
	VerifyFilecoinDeal(ctx context.Context, dealID string) (bool, error)

	// On-chain Registration
	RegisterReputationHash(ctx context.Context, runnerID, ipfsHash string) (string, error) // Returns tx hash
	VerifyOnChainHash(ctx context.Context, runnerID, ipfsHash string) (bool, error)

	// Public Verification
	GetPublicReputationData(ctx context.Context, runnerID string) (map[string]interface{}, error)
	VerifyReputationIntegrity(ctx context.Context, runnerID string) (bool, error)
}
