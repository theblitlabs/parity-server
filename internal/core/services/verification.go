package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/utils"
)

type VerificationService struct {
	taskRepo TaskRepository
}

func NewVerificationService(taskRepo TaskRepository) *VerificationService {
	return &VerificationService{
		taskRepo: taskRepo,
	}
}

func (s *VerificationService) VerifyTaskExecution(ctx context.Context, taskID string, imageHashVerified, commandHashVerified string) error {
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.taskRepo.Get(ctx, taskUUID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	if !utils.VerifyTaskHashes(task, imageHashVerified, commandHashVerified) {
		log.Error().
			Str("task_id", taskID).
			Str("expected_image_hash", task.ImageHash).
			Str("verified_image_hash", imageHashVerified).
			Str("expected_command_hash", task.CommandHash).
			Str("verified_command_hash", commandHashVerified).
			Msg("Hash verification failed")
		return fmt.Errorf("hash verification failed for task %s", taskID)
	}

	log.Info().
		Str("task_id", taskID).
		Msg("Hash verification successful")
	return nil
}

func (s *VerificationService) ProcessTaskResults(ctx context.Context, taskID string, results []*models.TaskResult) (*models.TaskResult, error) {
	log := gologger.WithComponent("verification")

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for task %s", taskID)
	}

	consensusHash, hasConsensus := utils.CheckConsensus(results, 0.67)
	if !hasConsensus {
		log.Warn().
			Str("task_id", taskID).
			Int("result_count", len(results)).
			Msg("No consensus reached for task results")

		for _, result := range results {
			result.VerificationStatus = "failed_consensus"
		}
		return nil, fmt.Errorf("no consensus reached for task %s", taskID)
	}

	var consensusResult *models.TaskResult
	for _, result := range results {
		if result.ResultHash == consensusHash {
			result.VerificationStatus = "verified"
			consensusResult = result
		} else {
			result.VerificationStatus = "rejected"
		}
	}

	log.Info().
		Str("task_id", taskID).
		Str("consensus_hash", consensusHash).
		Int("total_results", len(results)).
		Msg("Consensus reached for task results")

	return consensusResult, nil
}
