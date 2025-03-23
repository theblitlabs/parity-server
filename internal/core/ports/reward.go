package ports

import (
	"math/big"

	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

// ResourceMetrics represents the computing resources used during task execution
type ResourceMetrics struct {
	CPUSeconds      float64
	EstimatedCycles uint64
	MemoryGBHours   float64
	StorageGB       float64
	NetworkDataGB   float64
}

// RewardCalculator defines the interface for calculating rewards based on resource metrics
type RewardCalculator interface {
	CalculateReward(metrics ResourceMetrics) float64
}

// RewardClient defines the interface for distributing rewards to runners
type RewardClient interface {
	DistributeRewards(result *models.TaskResult) error
}

// StakeWallet defines the interface for interacting with the stake wallet
type StakeWallet interface {
	GetStakeInfo(deviceID string) (walletsdk.StakeInfo, error)
	TransferPayment(creator string, runner string, amount *big.Int) error
} 