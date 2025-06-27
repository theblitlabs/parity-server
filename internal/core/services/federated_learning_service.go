package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	requestmodels "github.com/theblitlabs/parity-server/internal/api/models"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type FederatedLearningService struct {
	flSessionRepo     ports.FLSessionRepository
	flRoundRepo       ports.FLRoundRepository
	flParticipantRepo ports.FLParticipantRepository
	runnerService     ports.RunnerService
	taskService       ports.TaskService
}

func NewFederatedLearningService(
	flSessionRepo ports.FLSessionRepository,
	flRoundRepo ports.FLRoundRepository,
	flParticipantRepo ports.FLParticipantRepository,
	runnerService ports.RunnerService,
	taskService ports.TaskService,
) *FederatedLearningService {
	return &FederatedLearningService{
		flSessionRepo:     flSessionRepo,
		flRoundRepo:       flRoundRepo,
		flParticipantRepo: flParticipantRepo,
		runnerService:     runnerService,
		taskService:       taskService,
	}
}

func (s *FederatedLearningService) CreateSession(ctx context.Context, req *requestmodels.CreateFLSessionRequest) (*models.FederatedLearningSession, error) {
	log := log.With().Str("component", "federated_learning_service").Logger()

	// Set default model config if not provided or incomplete
	if req.Config.ModelConfig == nil {
		req.Config.ModelConfig = make(map[string]interface{})
	}

	// Set sensible defaults for neural networks
	if _, exists := req.Config.ModelConfig["hidden_size"]; !exists {
		req.Config.ModelConfig["hidden_size"] = 64 // Good default for smaller datasets
	}

	config := models.FLConfig{
		AggregationMethod: req.Config.AggregationMethod,
		LearningRate:      req.Config.LearningRate,
		BatchSize:         req.Config.BatchSize,
		LocalEpochs:       req.Config.LocalEpochs,
		ClientSelection:   req.Config.ClientSelection,
		ModelConfig:       req.Config.ModelConfig,
		PrivacyConfig: models.PrivacyConfig{
			DifferentialPrivacy: req.Config.PrivacyConfig.DifferentialPrivacy,
			NoiseMultiplier:     req.Config.PrivacyConfig.NoiseMultiplier,
			L2NormClip:          req.Config.PrivacyConfig.L2NormClip,
			SecureAggregation:   req.Config.PrivacyConfig.SecureAggregation,
		},
	}

	minParticipants := req.MinParticipants
	if minParticipants == 0 {
		minParticipants = 2
	}

	trainingData := models.TrainingDataConfig{
		DatasetCID:    req.TrainingData.DatasetCID,
		DatasetSize:   req.TrainingData.DatasetSize,
		DataFormat:    req.TrainingData.DataFormat,
		Features:      req.TrainingData.Features,
		Labels:        req.TrainingData.Labels,
		SplitStrategy: req.TrainingData.SplitStrategy,
		Metadata:      req.TrainingData.Metadata,
	}

	session := models.NewFederatedLearningSession(
		req.Name,
		req.Description,
		req.ModelType,
		req.CreatorAddress,
		req.TotalRounds,
		minParticipants,
		config,
		trainingData,
	)

	if err := s.flSessionRepo.Create(ctx, session); err != nil {
		log.Error().Err(err).Msg("Failed to create FL session")
		return nil, fmt.Errorf("failed to create FL session: %w", err)
	}

	log.Info().
		Str("session_id", session.ID.String()).
		Str("name", session.Name).
		Str("status", string(session.Status)).
		Int("current_round", session.CurrentRound).
		Msg("Created federated learning session")

	return session, nil
}

func (s *FederatedLearningService) GetSession(ctx context.Context, sessionID uuid.UUID) (*models.FederatedLearningSession, error) {
	return s.flSessionRepo.GetByID(ctx, sessionID)
}

func (s *FederatedLearningService) ListSessions(ctx context.Context, creatorAddress string) ([]*models.FederatedLearningSession, error) {
	if creatorAddress != "" {
		return s.flSessionRepo.GetByCreator(ctx, creatorAddress)
	}
	return s.flSessionRepo.GetAll(ctx)
}

func (s *FederatedLearningService) autoSelectRunners(ctx context.Context, sessionID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Logger()

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	log.Info().
		Int("required_participants", session.MinParticipants).
		Msg("Searching for online runners")

	onlineRunners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get online runners")
		return fmt.Errorf("failed to get online runners: %w", err)
	}

	log.Info().
		Int("found_runners", len(onlineRunners)).
		Int("required_participants", session.MinParticipants).
		Msg("Found online runners")

	// Log details of found runners for debugging
	for i, runner := range onlineRunners {
		log.Info().
			Int("index", i).
			Str("device_id", runner.DeviceID).
			Str("status", string(runner.Status)).
			Str("wallet_address", runner.WalletAddress).
			Msg("Available runner")
	}

	// If we don't have enough online runners but we have minimum participants set to 1,
	// try to use any available runners instead of failing immediately
	if len(onlineRunners) < session.MinParticipants {
		if session.MinParticipants == 1 && len(onlineRunners) == 0 {
			// For single participant sessions, create a mock participant to allow testing
			log.Warn().Msg("No online runners found for single-participant session, using mock participant")
			mockRunnerID := "mock-runner-" + sessionID.String()[:8]
			if err := s.flSessionRepo.AddParticipant(ctx, sessionID, mockRunnerID); err != nil {
				log.Error().Err(err).Str("runner_id", mockRunnerID).Msg("Failed to add mock participant")
				return fmt.Errorf("failed to add mock participant: %w", err)
			}
			log.Info().Str("runner_id", mockRunnerID).Msg("Added mock participant for testing")
			return nil
		}

		log.Error().
			Int("required", session.MinParticipants).
			Int("available", len(onlineRunners)).
			Msg("Insufficient online runners")
		return fmt.Errorf("insufficient online runners: need %d, have %d", session.MinParticipants, len(onlineRunners))
	}

	selectedRunners := onlineRunners[:session.MinParticipants]

	successCount := 0
	for _, runner := range selectedRunners {
		if err := s.flSessionRepo.AddParticipant(ctx, sessionID, runner.DeviceID); err != nil {
			log.Error().Err(err).Str("runner_id", runner.DeviceID).Msg("Failed to add participant")
			continue
		}
		successCount++
		log.Info().Str("runner_id", runner.DeviceID).Msg("Auto-selected runner for FL session")
	}

	if successCount == 0 {
		return fmt.Errorf("failed to add any participants to session")
	}

	log.Info().Int("selected_count", successCount).Msg("Auto-selected runners for FL session")
	return nil
}

func (s *FederatedLearningService) StartSession(ctx context.Context, sessionID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Logger()

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session.Status != models.FLSessionStatusPending {
		return fmt.Errorf("session must be in pending status to start")
	}

	if err := s.autoSelectRunners(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to auto-select runners: %w", err)
	}

	session.Status = models.FLSessionStatusActive
	session.UpdatedAt = time.Now()

	if err := s.flSessionRepo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	if err := s.StartNextRound(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to start first round: %w", err)
	}

	log.Info().Msg("Started FL session")
	return nil
}

func (s *FederatedLearningService) StartNextRound(ctx context.Context, sessionID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Logger()

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session.CurrentRound >= session.TotalRounds {
		return s.CompleteSession(ctx, sessionID)
	}

	nextRound := session.CurrentRound + 1
	round := models.NewFederatedLearningRound(sessionID, nextRound)
	round.Status = models.FLRoundStatusCollecting

	if err := s.flRoundRepo.Create(ctx, round); err != nil {
		return fmt.Errorf("failed to create round: %w", err)
	}

	session.CurrentRound = nextRound
	session.UpdatedAt = time.Now()

	if err := s.flSessionRepo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	if err := s.assignParticipants(ctx, sessionID, round.ID); err != nil {
		return fmt.Errorf("failed to assign participants: %w", err)
	}

	log.Info().
		Int("round_number", nextRound).
		Msg("Started new FL round")

	return nil
}

func (s *FederatedLearningService) assignParticipants(ctx context.Context, sessionID, roundID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Str("round_id", roundID.String()).
		Logger()

	participants, err := s.flSessionRepo.GetParticipants(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get participants: %w", err)
	}

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	log.Info().
		Int("participant_count", len(participants)).
		Msg("Assigning participants to FL round")

	// If no participants were pre-registered (e.g., for testing scenarios),
	// try to auto-assign online runners
	if len(participants) == 0 {
		log.Warn().Msg("No participants found, attempting to auto-assign online runners")

		onlineRunners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get online runners for auto-assignment")
		} else if len(onlineRunners) > 0 {
			// Take the first available runner
			selectedRunner := onlineRunners[0]
			if err := s.flSessionRepo.AddParticipant(ctx, sessionID, selectedRunner.DeviceID); err != nil {
				log.Error().Err(err).Str("runner_id", selectedRunner.DeviceID).Msg("Failed to auto-add participant")
			} else {
				participants = append(participants, selectedRunner.DeviceID)
				log.Info().Str("runner_id", selectedRunner.DeviceID).Msg("Auto-added online runner as participant")
			}
		}

		// If still no participants, create a task anyway for any available runners
		if len(participants) == 0 {
			log.Info().Msg("No participants registered, creating open assignment")
			// We'll create a round participant entry that can be matched by any runner
			openParticipant := models.NewFLRoundParticipant(roundID, "open-assignment")
			if err := s.flParticipantRepo.Create(ctx, openParticipant); err != nil {
				log.Error().Err(err).Msg("Failed to create open assignment participant")
			} else {
				log.Info().Msg("Created open assignment for any available runner")
			}
		}
	}

	successCount := 0
	for _, participantID := range participants {
		// Check if runner exists and is online
		runner, err := s.runnerService.GetRunner(ctx, participantID)
		if err != nil {
			log.Warn().Str("runner_id", participantID).Msg("Failed to get runner, but creating participant anyway")
		} else if runner.Status != models.RunnerStatusOnline {
			log.Warn().Str("runner_id", participantID).Msg("Runner not online, but creating participant anyway")
		}

		participant := models.NewFLRoundParticipant(roundID, participantID)
		if err := s.flParticipantRepo.Create(ctx, participant); err != nil {
			log.Error().Err(err).Str("runner_id", participantID).Msg("Failed to create participant")
			continue
		}

		log.Info().
			Str("runner_id", participantID).
			Msg("Created FL round participant")

		if err := s.sendTrainingTask(ctx, session, roundID, participantID); err != nil {
			log.Error().Err(err).Str("runner_id", participantID).Msg("Failed to send training task")
			// Continue instead of returning error - the participant is registered even if task sending fails
			continue
		}

		successCount++
		log.Info().
			Str("runner_id", participantID).
			Msg("Successfully sent training task to runner")
	}

	// Even if no specific participants succeeded, we might have created an open assignment
	if successCount == 0 && len(participants) > 0 {
		return fmt.Errorf("failed to assign any registered participants to the round")
	}

	log.Info().
		Int("success_count", successCount).
		Int("total_participants", len(participants)).
		Msg("Participant assignment completed")

	return nil
}

func (s *FederatedLearningService) sendTrainingTask(ctx context.Context, session *models.FederatedLearningSession, roundID uuid.UUID, runnerID string) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", session.ID.String()).
		Str("round_id", roundID.String()).
		Str("runner_id", runnerID).
		Logger()

	// Set default values for missing fields
	dataFormat := session.TrainingData.DataFormat
	if dataFormat == "" {
		dataFormat = "csv"
	}

	// Validate required fields
	if session.TrainingData.DatasetCID == "" {
		log.Error().Msg("Dataset CID is missing from session training data")
		return fmt.Errorf("dataset CID is required but missing from session")
	}

	// Get all participants to determine partition configuration
	participants, err := s.flSessionRepo.GetParticipants(ctx, session.ID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get participants, using default partitioning")
		participants = []string{runnerID} // Fallback to single participant
	}

	// Find the index of current runner in participants list
	runnerIndex := 0
	for i, participantID := range participants {
		if participantID == runnerID {
			runnerIndex = i
			break
		}
	}

	// Create partition configuration based on session settings
	partitionConfig := map[string]interface{}{
		"strategy":      session.TrainingData.SplitStrategy,
		"total_parts":   len(participants),
		"part_index":    runnerIndex,
		"alpha":         0.5, // Default Dirichlet parameter for non-IID
		"min_samples":   50,  // Minimum samples per participant
		"overlap_ratio": 0.0, // No overlap by default
	}

	// Override with session-specific partition settings if available
	if session.TrainingData.Metadata != nil {
		if alpha, ok := session.TrainingData.Metadata["alpha"].(float64); ok {
			partitionConfig["alpha"] = alpha
		}
		if minSamples, ok := session.TrainingData.Metadata["min_samples"].(float64); ok {
			partitionConfig["min_samples"] = int(minSamples)
		}
		if overlapRatio, ok := session.TrainingData.Metadata["overlap_ratio"].(float64); ok {
			partitionConfig["overlap_ratio"] = overlapRatio
		}
	}

	// Set default split strategy if not specified
	if partitionConfig["strategy"] == "" || partitionConfig["strategy"] == nil {
		partitionConfig["strategy"] = "random"
	}

	config := map[string]interface{}{
		"session_id":       session.ID.String(),
		"round_id":         roundID.String(),
		"model_type":       session.ModelType,
		"dataset_cid":      session.TrainingData.DatasetCID,
		"data_format":      dataFormat,
		"model_config":     session.Config.ModelConfig,
		"partition_config": partitionConfig,
		"train_config": map[string]interface{}{
			"epochs":        session.Config.LocalEpochs,
			"batch_size":    session.Config.BatchSize,
			"learning_rate": session.Config.LearningRate,
		},
		"output_format": "json",
		"global_model":  session.GlobalModel,
	}

	log.Info().
		Str("dataset_cid", session.TrainingData.DatasetCID).
		Str("data_format", dataFormat).
		Str("model_type", session.ModelType).
		Str("split_strategy", fmt.Sprintf("%v", partitionConfig["strategy"])).
		Int("total_participants", len(participants)).
		Int("runner_index", runnerIndex).
		Msg("Creating FL training task with partitioned data configuration")

	configData, err := json.Marshal(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal FL task config")
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	task := &models.Task{
		ID:              uuid.New(),
		Title:           fmt.Sprintf("FL Training - Session %s Round %d (Partition %d/%d)", session.Name, session.CurrentRound, runnerIndex+1, len(participants)),
		Description:     fmt.Sprintf("Federated learning training task for session %s with %s data partitioning", session.Name, partitionConfig["strategy"]),
		Type:            models.TaskTypeFederatedLearning,
		Status:          models.TaskStatusPending,
		Config:          configData,
		CreatorAddress:  session.CreatorAddress,
		CreatorDeviceID: "fl-coordinator",
		RunnerID:        runnerID,
		Nonce:           fmt.Sprintf("fl-%s-%d-%d", session.ID.String(), session.CurrentRound, runnerIndex),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("task_type", string(task.Type)).
		Msg("Creating FL training task")

	if err := s.taskService.CreateTask(ctx, task); err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Msg("Failed to create FL training task")
		return fmt.Errorf("failed to create training task: %w", err)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Msg("FL training task created successfully")

	return nil
}

func (s *FederatedLearningService) SubmitModelUpdate(ctx context.Context, req *requestmodels.SubmitModelUpdateRequest) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", req.SessionID).
		Str("round_id", req.RoundID).
		Str("runner_id", req.RunnerID).
		Logger()

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	roundID, err := uuid.Parse(req.RoundID)
	if err != nil {
		return fmt.Errorf("invalid round ID: %w", err)
	}

	participant, err := s.flParticipantRepo.GetByRoundAndRunner(ctx, roundID, req.RunnerID)
	if err != nil {
		log.Warn().Err(err).Msg("Participant not found, attempting dynamic registration")

		// Try to register the runner as a participant dynamically
		newParticipant := models.NewFLRoundParticipant(roundID, req.RunnerID)
		newParticipant.Status = models.FLParticipantStatusTraining // Set status to training since they're submitting updates

		if createErr := s.flParticipantRepo.Create(ctx, newParticipant); createErr != nil {
			log.Error().Err(createErr).Msg("Failed to create participant dynamically")
			return fmt.Errorf("failed to get participant and dynamic registration failed: %w", err)
		}

		// Also add them to the session participants if not already there
		if addErr := s.flSessionRepo.AddParticipant(ctx, sessionID, req.RunnerID); addErr != nil {
			log.Warn().Err(addErr).Msg("Failed to add participant to session (may already exist)")
		}

		participant = newParticipant
		log.Info().Msg("Successfully registered participant dynamically")
	}

	if participant.Status != models.FLParticipantStatusAssigned &&
		participant.Status != models.FLParticipantStatusTraining {
		log.Warn().
			Str("current_status", string(participant.Status)).
			Msg("Participant not in expected status, but allowing update submission")
		// Don't return error - allow the submission to proceed
	}

	modelUpdate := models.ModelUpdate{
		Gradients:  req.Gradients,
		Weights:    req.Weights,
		UpdateType: req.UpdateType,
		DataSize:   req.DataSize,
		Loss:       req.Loss,
		Accuracy:   req.Accuracy,
		Metadata:   req.Metadata,
	}

	updateData, err := json.Marshal(modelUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal model update: %w", err)
	}

	participant.ModelUpdate = updateData
	participant.Status = models.FLParticipantStatusCompleted
	participant.DataSize = req.DataSize
	participant.UpdatedAt = time.Now()
	now := time.Now()
	participant.CompletedAt = &now

	if req.TrainingTime > 0 {
		metrics := map[string]interface{}{
			"training_time_ms": req.TrainingTime,
			"loss":             req.Loss,
			"accuracy":         req.Accuracy,
		}
		if req.PrivacyMetrics != nil {
			metrics["privacy_metrics"] = req.PrivacyMetrics
		}
		metricsData, _ := json.Marshal(metrics)
		participant.TrainingMetrics = metricsData
	}

	if err := s.flParticipantRepo.Update(ctx, participant); err != nil {
		return fmt.Errorf("failed to update participant: %w", err)
	}

	log.Info().
		Float64("loss", req.Loss).
		Float64("accuracy", req.Accuracy).
		Int("data_size", req.DataSize).
		Msg("Model update submitted successfully")

	if err := s.CheckRoundCompletion(ctx, sessionID, roundID); err != nil {
		log.Error().Err(err).Msg("Failed to check round completion")
	}

	return nil
}

func (s *FederatedLearningService) CheckRoundCompletion(ctx context.Context, sessionID, roundID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Str("round_id", roundID.String()).
		Logger()

	completed, err := s.flParticipantRepo.CountCompleted(ctx, roundID)
	if err != nil {
		return fmt.Errorf("failed to count completed participants: %w", err)
	}

	total, err := s.flParticipantRepo.CountTotal(ctx, roundID)
	if err != nil {
		return fmt.Errorf("failed to count total participants: %w", err)
	}

	log.Debug().
		Int("completed", completed).
		Int("total", total).
		Msg("Checking round completion status")

	if completed >= total {
		return s.AggregateRound(ctx, sessionID, roundID)
	}

	return nil
}

func (s *FederatedLearningService) AggregateRound(ctx context.Context, sessionID, roundID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Str("round_id", roundID.String()).
		Logger()

	round, err := s.flRoundRepo.GetByID(ctx, roundID)
	if err != nil {
		return fmt.Errorf("failed to get round: %w", err)
	}

	if round.Status != models.FLRoundStatusCollecting {
		return fmt.Errorf("round not in collecting status")
	}

	round.Status = models.FLRoundStatusAggregating
	round.UpdatedAt = time.Now()

	if err := s.flRoundRepo.Update(ctx, round); err != nil {
		return fmt.Errorf("failed to update round status: %w", err)
	}

	participants, err := s.flParticipantRepo.GetByRound(ctx, roundID)
	if err != nil {
		return fmt.Errorf("failed to get participants: %w", err)
	}

	aggregatedModel, aggregatedGradients, aggregatedWeights, globalMetrics, err := s.performAggregation(participants)
	if err != nil {
		return fmt.Errorf("failed to perform aggregation: %w", err)
	}

	aggregation := &models.AggregationResult{
		Method:           "federated_averaging",
		AggregatedModel:  aggregatedModel,
		Gradients:        aggregatedGradients,
		Weights:          aggregatedWeights,
		GlobalMetrics:    *globalMetrics,
		RoundSummary:     fmt.Sprintf("Round %d completed with %d participants", round.RoundNumber, len(participants)),
		ParticipantCount: len(participants),
	}

	aggregationData, err := json.Marshal(aggregation)
	if err != nil {
		return fmt.Errorf("failed to marshal aggregation: %w", err)
	}

	round.Aggregation = aggregation
	round.Status = models.FLRoundStatusCompleted
	round.UpdatedAt = time.Now()
	now := time.Now()
	round.CompletedAt = &now

	if err := s.flRoundRepo.Update(ctx, round); err != nil {
		return fmt.Errorf("failed to update round: %w", err)
	}

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.GlobalModel = json.RawMessage(aggregationData)
	session.UpdatedAt = time.Now()

	if err := s.flSessionRepo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session with new global model: %w", err)
	}

	log.Info().
		Int("participants", len(participants)).
		Msg("Round aggregation completed")

	if session.CurrentRound < session.TotalRounds {
		return s.StartNextRound(ctx, sessionID)
	}

	return s.CompleteSession(ctx, sessionID)
}

func (s *FederatedLearningService) performAggregation(participants []*models.FLRoundParticipant) (map[string][]float64, map[string][]float64, map[string][]float64, *models.GlobalMetrics, error) {
	if len(participants) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no participants to aggregate")
	}

	aggregatedGradients := make(map[string][]float64)
	aggregatedWeights := make(map[string][]float64)
	totalWeight := 0.0
	totalLoss := 0.0
	totalAccuracy := 0.0
	validParticipants := 0

	// First pass: aggregate gradients and collect weights
	for _, participant := range participants {
		if participant.ModelUpdate == nil {
			continue
		}

		var modelUpdate models.ModelUpdate
		if err := json.Unmarshal(participant.ModelUpdate, &modelUpdate); err != nil {
			log.Error().Err(err).Str("participant_id", participant.ID.String()).Msg("Failed to unmarshal model update")
			continue
		}

		weight := float64(participant.DataSize)
		totalWeight += weight

		// Aggregate gradients
		for layerName, gradients := range modelUpdate.Gradients {
			if _, exists := aggregatedGradients[layerName]; !exists {
				aggregatedGradients[layerName] = make([]float64, len(gradients))
			}

			for i, gradient := range gradients {
				if i < len(aggregatedGradients[layerName]) {
					aggregatedGradients[layerName][i] += gradient * weight
				}
			}
		}

		// Collect weights if available
		if modelUpdate.Weights != nil {
			for layerName, weights := range modelUpdate.Weights {
				if _, exists := aggregatedWeights[layerName]; !exists {
					aggregatedWeights[layerName] = make([]float64, len(weights))
				}

				for i, modelWeight := range weights {
					if i < len(aggregatedWeights[layerName]) {
						aggregatedWeights[layerName][i] += modelWeight * weight
					}
				}
			}
		}

		totalLoss += modelUpdate.Loss * weight
		totalAccuracy += modelUpdate.Accuracy * weight
		validParticipants++
	}

	if totalWeight == 0 {
		return nil, nil, nil, nil, fmt.Errorf("total weight is zero")
	}

	// Normalize aggregated values
	for layerName := range aggregatedGradients {
		for i := range aggregatedGradients[layerName] {
			aggregatedGradients[layerName][i] /= totalWeight
		}
	}

	for layerName := range aggregatedWeights {
		for i := range aggregatedWeights[layerName] {
			aggregatedWeights[layerName][i] /= totalWeight
		}
	}

	// If we have weights, use them; otherwise, use the aggregated gradients as the final model
	finalModel := aggregatedWeights
	if len(finalModel) == 0 {
		finalModel = aggregatedGradients
	}

	avgLoss := totalLoss / totalWeight
	avgAccuracy := totalAccuracy / totalWeight
	variance := s.calculateVariance(participants, avgLoss)
	convergence := s.calculateConvergence(avgLoss, variance)

	metrics := &models.GlobalMetrics{
		AverageLoss:     avgLoss,
		AverageAccuracy: avgAccuracy,
		Variance:        variance,
		Convergence:     convergence,
	}

	return finalModel, aggregatedGradients, aggregatedWeights, metrics, nil
}

func (s *FederatedLearningService) calculateVariance(participants []*models.FLRoundParticipant, avgLoss float64) float64 {
	if len(participants) <= 1 {
		return 0.0
	}

	sumSquaredDiff := 0.0
	validCount := 0

	for _, participant := range participants {
		if participant.ModelUpdate == nil {
			continue
		}

		var modelUpdate models.ModelUpdate
		if err := json.Unmarshal(participant.ModelUpdate, &modelUpdate); err != nil {
			continue
		}

		diff := modelUpdate.Loss - avgLoss
		sumSquaredDiff += diff * diff
		validCount++
	}

	if validCount <= 1 {
		return 0.0
	}

	return sumSquaredDiff / float64(validCount-1)
}

func (s *FederatedLearningService) calculateConvergence(avgLoss, variance float64) float64 {
	if variance == 0 {
		return 1.0
	}
	return math.Max(0, 1.0-variance/avgLoss)
}

func (s *FederatedLearningService) CompleteSession(ctx context.Context, sessionID uuid.UUID) error {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Logger()

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.Status = models.FLSessionStatusCompleted
	session.UpdatedAt = time.Now()
	now := time.Now()
	session.CompletedAt = &now

	if err := s.flSessionRepo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	log.Info().Msg("FL session completed")
	return nil
}

func (s *FederatedLearningService) GetTrainedModel(ctx context.Context, sessionID uuid.UUID) (map[string]interface{}, error) {
	log := log.With().
		Str("component", "federated_learning_service").
		Str("session_id", sessionID.String()).
		Logger()

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.GlobalModel == nil {
		return nil, fmt.Errorf("no trained model available - session may not be completed or no training rounds completed")
	}

	var modelData map[string]interface{}
	if err := json.Unmarshal(session.GlobalModel, &modelData); err != nil {
		return nil, fmt.Errorf("failed to parse model data: %w", err)
	}

	result := map[string]interface{}{
		"session_id":   session.ID.String(),
		"session_name": session.Name,
		"model_type":   session.ModelType,
		"status":       string(session.Status),
		"total_rounds": session.TotalRounds,
		"completed_at": session.CompletedAt,
		"model_data":   modelData,
	}

	log.Info().Msg("Trained model retrieved successfully")
	return result, nil
}
