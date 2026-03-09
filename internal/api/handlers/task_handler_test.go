package handlers

import (
	"testing"

	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

func TestCheckStakeBalanceSkipsWhenMinimumStakeDisabled(t *testing.T) {
	handler := &TaskHandler{
		config: &config.Config{
			Reputation: config.ReputationConfig{
				MinimumStake: 0,
			},
		},
	}

	task := models.NewTask()
	task.CreatorDeviceID = "device-1"

	if err := handler.checkStakeBalance(task); err != nil {
		t.Fatalf("expected stake validation to be skipped, got error: %v", err)
	}
}
