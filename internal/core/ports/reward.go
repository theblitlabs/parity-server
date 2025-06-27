package ports

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type ResourceMetrics struct {
	CPUSeconds      float64
	EstimatedCycles uint64
	MemoryGBHours   float64
	StorageGB       float64
	NetworkDataGB   float64
}

type RewardCalculator interface {
	CalculateReward(metrics ResourceMetrics) float64
}

type RewardClient interface {
	DistributeRewards(result *models.TaskResult) error
}

type StakeWallet interface {
	GetStakeInfo(deviceID string) (walletsdk.StakeInfo, error)
	TransferPayment(creator string, runner string, amount *big.Int) (*types.Transaction, error)
}
