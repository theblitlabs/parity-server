package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type LLMService struct {
	promptRepo    ports.PromptRepository
	billingRepo   ports.BillingRepository
	runnerRepo    ports.RunnerRepository
	runnerService *RunnerService
	taskQueue     *TaskQueue
}

func NewLLMService(
	promptRepo ports.PromptRepository,
	billingRepo ports.BillingRepository,
	runnerRepo ports.RunnerRepository,
	runnerService *RunnerService,
	taskQueue *TaskQueue,
) *LLMService {
	return &LLMService{
		promptRepo:    promptRepo,
		billingRepo:   billingRepo,
		runnerRepo:    runnerRepo,
		runnerService: runnerService,
		taskQueue:     taskQueue,
	}
}

func (s *LLMService) SubmitPrompt(ctx context.Context, clientID, prompt, modelName, creatorAddress string) (*models.PromptRequest, error) {
	log := gologger.WithComponent("llm_service")

	runner, err := s.findAvailableRunner(ctx, modelName)
	if err != nil {
		log.Error().Err(err).Str("model_name", modelName).Msg("No available runner found for model")
		return nil, fmt.Errorf("no available runner found for model %s: %w", modelName, err)
	}

	promptReq := models.NewPromptRequest(clientID, prompt, modelName, creatorAddress)
	promptReq.RunnerID = runner.DeviceID
	promptReq.Status = models.PromptStatusProcessing

	if err := s.promptRepo.Create(ctx, promptReq); err != nil {
		log.Error().Err(err).Msg("Failed to create prompt request")
		return nil, fmt.Errorf("failed to create prompt request: %w", err)
	}

	// Forward prompt to runner asynchronously
	go func() {
		// Use background context to avoid cancellation when HTTP request ends
		bgCtx := context.Background()
		if err := s.runnerService.ForwardPromptToRunner(bgCtx, runner.DeviceID, promptReq); err != nil {
			// Mark prompt as failed when webhook delivery fails
			log.Error().Err(err).Str("runner_id", runner.DeviceID).Msg("Failed to forward prompt to runner - marking prompt as failed")

			// Update prompt status to failed
			now := time.Now()
			promptReq.Status = models.PromptStatusFailed
			promptReq.CompletedAt = &now
			if updateErr := s.promptRepo.Update(bgCtx, promptReq); updateErr != nil {
				log.Error().Err(updateErr).Str("prompt_id", promptReq.ID.String()).Msg("Failed to update prompt status to failed")
			}
		}
	}()

	log.Info().
		Str("prompt_id", promptReq.ID.String()).
		Str("model_name", modelName).
		Str("runner_id", runner.DeviceID).
		Msg("Prompt submitted successfully")

	return promptReq, nil
}

func (s *LLMService) CompletePrompt(ctx context.Context, promptID uuid.UUID, response string, promptTokens, responseTokens int, inferenceTime int64) error {
	log := gologger.WithComponent("llm_service")

	promptReq, err := s.promptRepo.GetByID(ctx, promptID)
	if err != nil {
		log.Error().Err(err).Str("prompt_id", promptID.String()).Msg("Failed to get prompt request")
		return fmt.Errorf("failed to get prompt request: %w", err)
	}

	now := time.Now()
	promptReq.Response = response
	promptReq.Status = models.PromptStatusCompleted
	promptReq.CompletedAt = &now

	if err := s.promptRepo.Update(ctx, promptReq); err != nil {
		log.Error().Err(err).Str("prompt_id", promptID.String()).Msg("Failed to update prompt request")
		return fmt.Errorf("failed to update prompt request: %w", err)
	}

	// Free up the runner by clearing its TaskID
	if promptReq.RunnerID != "" {
		runner, err := s.runnerRepo.GetRunnerByDeviceID(ctx, promptReq.RunnerID)
		if err != nil {
			log.Error().Err(err).Str("runner_id", promptReq.RunnerID).Msg("Failed to get runner for cleanup")
		} else {
			runner.TaskID = nil
			if _, err := s.runnerService.UpdateRunner(ctx, runner); err != nil {
				log.Error().Err(err).Str("runner_id", promptReq.RunnerID).Msg("Failed to clear runner TaskID")
			} else {
				log.Info().Str("runner_id", promptReq.RunnerID).Msg("Runner freed after prompt completion")
			}
		}
	}

	metric := models.NewBillingMetric(
		promptReq.ClientID,
		promptID,
		promptReq.ModelName,
		promptTokens,
		responseTokens,
		inferenceTime,
	)

	if err := s.billingRepo.Create(ctx, metric); err != nil {
		log.Error().Err(err).Str("prompt_id", promptID.String()).Msg("Failed to create billing metric")
		return fmt.Errorf("failed to create billing metric: %w", err)
	}

	log.Info().
		Str("prompt_id", promptID.String()).
		Int("total_tokens", promptTokens+responseTokens).
		Int64("inference_time_ms", inferenceTime).
		Msg("Prompt completed with billing metrics")

	return nil
}

func (s *LLMService) GetPrompt(ctx context.Context, promptID uuid.UUID) (*models.PromptRequest, error) {
	return s.promptRepo.GetByID(ctx, promptID)
}

func (s *LLMService) ListPrompts(ctx context.Context, clientID string, limit, offset int) ([]*models.PromptRequest, error) {
	return s.promptRepo.ListByClientID(ctx, clientID, limit, offset)
}

func (s *LLMService) GetBillingMetrics(ctx context.Context, clientID string) (*models.BillingMetric, error) {
	return s.billingRepo.GetAggregatedMetrics(ctx, clientID)
}

func (s *LLMService) GetAvailableModels(ctx context.Context) ([]models.ModelCapability, error) {
	runners, err := s.runnerRepo.GetOnlineRunners(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get online runners: %w", err)
	}

	// Use a map to deduplicate models
	modelMap := make(map[string]models.ModelCapability)

	for _, runner := range runners {
		if runner.Status == models.RunnerStatusOnline {
			for _, capability := range runner.ModelCapabilities {
				if capability.IsLoaded {
					// Use the model name as key to avoid duplicates
					modelMap[capability.ModelName] = capability
				}
			}
		}
	}

	// Convert map to slice
	models := make([]models.ModelCapability, 0, len(modelMap))
	for _, model := range modelMap {
		models = append(models, model)
	}

	return models, nil
}

func (s *LLMService) findAvailableRunner(ctx context.Context, modelName string) (*models.Runner, error) {
	runners, err := s.runnerRepo.GetOnlineRunners(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get online runners: %w", err)
	}

	for _, runner := range runners {
		for _, capability := range runner.ModelCapabilities {
			// Check for exact match first
			if capability.ModelName == modelName && capability.IsLoaded {
				if runner.Status == models.RunnerStatusOnline {
					return runner, nil
				}
			}

			// Check for base model name match (e.g., "qwen3" matches "qwen3:latest", "qwen3:8b", etc.)
			if matchesBaseModel(capability.ModelName, modelName) && capability.IsLoaded {
				if runner.Status == models.RunnerStatusOnline {
					return runner, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no available runner found for model %s", modelName)
}

// matchesBaseModel checks if a model capability matches the requested model name
// Supports matching "qwen3" against "qwen3:latest", "qwen3:8b", etc.
func matchesBaseModel(capabilityModel, requestedModel string) bool {
	// Import strings package if not already imported
	if capabilityModel == requestedModel {
		return true
	}

	// Check if capability model starts with requested model followed by ":"
	// This matches "qwen3" with "qwen3:latest", "qwen3:8b", etc.
	if len(capabilityModel) > len(requestedModel) {
		if capabilityModel[:len(requestedModel)] == requestedModel &&
			capabilityModel[len(requestedModel)] == ':' {
			return true
		}
	}

	// Check if requested model starts with capability model followed by ":"
	// This matches "qwen3:8b" with "qwen3", etc.
	if len(requestedModel) > len(capabilityModel) {
		if requestedModel[:len(capabilityModel)] == capabilityModel &&
			requestedModel[len(capabilityModel)] == ':' {
			return true
		}
	}

	return false
}

func (s *LLMService) CreatePrompt(ctx context.Context, clientID, prompt, modelName, creatorAddress string) (*models.PromptRequest, error) {
	log := gologger.WithComponent("llm_service")

	if prompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	if modelName == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	if creatorAddress == "" {
		return nil, fmt.Errorf("creator address cannot be empty")
	}

	promptReq := models.NewPromptRequest(clientID, prompt, modelName, creatorAddress)

	// Try to get available runner for model
	runnerID, err := s.runnerService.GetAvailableRunnerForModel(ctx, modelName)
	if err != nil {
		// No runner available, queue the task instead of failing
		log.Info().
			Str("model_name", modelName).
			Str("prompt_id", promptReq.ID.String()).
			Msg("No runner available, queuing task for later processing")

		promptReq.Status = models.PromptStatusQueued

		// Create prompt in DB with queued status
		if err := s.promptRepo.Create(ctx, promptReq); err != nil {
			return nil, fmt.Errorf("failed to create prompt request: %w", err)
		}

		// Add to task queue
		s.taskQueue.QueueTask(promptReq.ID, modelName)

		log.Info().
			Str("prompt_id", promptReq.ID.String()).
			Str("model_name", modelName).
			Msg("Prompt queued successfully")

		return promptReq, nil
	}

	// Runner is available, process immediately
	promptReq.RunnerID = runnerID
	promptReq.Status = models.PromptStatusProcessing

	// Create prompt in DB
	if err := s.promptRepo.Create(ctx, promptReq); err != nil {
		return nil, fmt.Errorf("failed to create prompt request: %w", err)
	}

	// Forward prompt to runner asynchronously
	go func() {
		// Use background context to avoid cancellation when HTTP request ends
		bgCtx := context.Background()
		if err := s.runnerService.ForwardPromptToRunner(bgCtx, runnerID, promptReq); err != nil {
			// Mark prompt as failed when webhook delivery fails
			log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to forward prompt to runner - marking prompt as failed")

			// Update prompt status to failed
			now := time.Now()
			promptReq.Status = models.PromptStatusFailed
			promptReq.CompletedAt = &now
			if updateErr := s.promptRepo.Update(bgCtx, promptReq); updateErr != nil {
				log.Error().Err(updateErr).Str("prompt_id", promptReq.ID.String()).Msg("Failed to update prompt status to failed")
			}

			// Free up the runner by clearing its TaskID
			runner, err := s.runnerRepo.GetRunnerByDeviceID(bgCtx, runnerID)
			if err != nil {
				log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to get runner for cleanup after failure")
			} else {
				runner.TaskID = nil
				if _, err := s.runnerService.UpdateRunner(bgCtx, runner); err != nil {
					log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to clear runner TaskID after failure")
				} else {
					log.Info().Str("runner_id", runnerID).Msg("Runner freed after prompt failure")
				}
			}
		}
	}()

	log.Info().
		Str("prompt_id", promptReq.ID.String()).
		Str("model_name", modelName).
		Str("runner_id", runnerID).
		Msg("Prompt submitted successfully")

	return promptReq, nil
}
