package services

import (
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type RewardCalculator struct {
	cpuCostPerSecond     float64
	memoryCostPerGBHour  float64
	storageCostPerGB     float64
	networkCostPerGB     float64
	cyclesCostPerMillion float64
}

func NewRewardCalculator() ports.RewardCalculator {
	return &RewardCalculator{
		cpuCostPerSecond:     0.00001,  // $0.00001 per CPU second
		memoryCostPerGBHour:  0.00005,  // $0.00005 per GB-hour
		storageCostPerGB:     0.0001,   // $0.0001 per GB
		networkCostPerGB:     0.0001,   // $0.0001 per GB
		cyclesCostPerMillion: 0.000001, // $0.000001 per million cycles
	}
}

func (rc *RewardCalculator) CalculateReward(metrics ports.ResourceMetrics) float64 {
	cpuCost := metrics.CPUSeconds * rc.cpuCostPerSecond
	memoryCost := metrics.MemoryGBHours * rc.memoryCostPerGBHour
	storageCost := metrics.StorageGB * rc.storageCostPerGB
	networkCost := metrics.NetworkDataGB * rc.networkCostPerGB
	cyclesCost := float64(metrics.EstimatedCycles) / 1_000_000.0 * rc.cyclesCostPerMillion

	totalCost := (cpuCost + memoryCost + storageCost + networkCost + cyclesCost) * 1.2

	if totalCost < 0.0001 {
		totalCost = 0.0001
	}

	return totalCost
}