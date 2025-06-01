package services

import (
	"context"
	"fmt"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/utils"
)

type ConsensusService struct {
	taskRepo            TaskRepository
	verificationService *VerificationService
}

func NewConsensusService(taskRepo TaskRepository, verificationService *VerificationService) *ConsensusService {
	return &ConsensusService{
		taskRepo:            taskRepo,
		verificationService: verificationService,
	}
}

func (s *ConsensusService) ProcessTaskConsensus(ctx context.Context, taskID string, results []*models.TaskResult) (*models.TaskResult, error) {
	log := gologger.WithComponent("consensus")

	if len(results) == 0 {
		return nil, fmt.Errorf("no results provided for consensus")
	}

	log.Info().
		Str("task_id", taskID).
		Int("result_count", len(results)).
		Msg("Processing task consensus")

	consensusResult, err := s.verificationService.ProcessTaskResults(ctx, taskID, results)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", taskID).
			Msg("Failed to process task results consensus")
		return nil, err
	}

	if consensusResult == nil {
		log.Warn().
			Str("task_id", taskID).
			Msg("No consensus reached for task")
		return nil, fmt.Errorf("no consensus reached for task %s", taskID)
	}

	log.Info().
		Str("task_id", taskID).
		Str("consensus_result_id", consensusResult.ID.String()).
		Str("result_hash", consensusResult.ResultHash).
		Msg("Consensus reached for task")

	return consensusResult, nil
}

func (s *ConsensusService) ValidateResultIntegrity(result *models.TaskResult) error {
	log := gologger.WithComponent("consensus")

	if result.ResultHash == "" {
		return fmt.Errorf("result hash is missing")
	}

	expectedHash := utils.ComputeResultHash(result.Output, result.Error, result.ExitCode)
	if result.ResultHash != expectedHash {
		log.Error().
			Str("result_id", result.ID.String()).
			Str("expected_hash", expectedHash).
			Str("actual_hash", result.ResultHash).
			Msg("Result hash mismatch detected")
		return fmt.Errorf("result hash mismatch: expected %s, got %s", expectedHash, result.ResultHash)
	}

	log.Debug().
		Str("result_id", result.ID.String()).
		Str("result_hash", result.ResultHash).
		Msg("Result integrity validated")

	return nil
}

func (s *ConsensusService) CheckMinimumRunners(results []*models.TaskResult, minRunners int) error {
	if len(results) < minRunners {
		return fmt.Errorf("insufficient runners: got %d, minimum required %d", len(results), minRunners)
	}
	return nil
}
