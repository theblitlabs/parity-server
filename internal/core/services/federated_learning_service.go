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

	if len(onlineRunners) < session.MinParticipants {
		log.Error().
			Int("required", session.MinParticipants).
			Int("available", len(onlineRunners)).
			Msg("Insufficient online runners")
		return fmt.Errorf("insufficient online runners: need %d, have %d", session.MinParticipants, len(onlineRunners))
	}

	selectedRunners := onlineRunners[:session.MinParticipants]

	for _, runner := range selectedRunners {
		if err := s.flSessionRepo.AddParticipant(ctx, sessionID, runner.DeviceID); err != nil {
			log.Error().Err(err).Str("runner_id", runner.DeviceID).Msg("Failed to add participant")
			continue
		}
		log.Info().Str("runner_id", runner.DeviceID).Msg("Auto-selected runner for FL session")
	}

	log.Info().Int("selected_count", len(selectedRunners)).Msg("Auto-selected runners for FL session")
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

	successCount := 0
	for _, participantID := range participants {
		runner, err := s.runnerService.GetRunner(ctx, participantID)
		if err != nil {
			log.Warn().Str("runner_id", participantID).Msg("Failed to get runner, skipping")
			continue
		}

		if runner.Status != models.RunnerStatusOnline {
			log.Warn().Str("runner_id", participantID).Msg("Runner not online, skipping")
			continue
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
			// Don't continue here - we want to know about task creation failures
			return fmt.Errorf("failed to send training task to runner %s: %w", participantID, err)
		}

		successCount++
		log.Info().
			Str("runner_id", participantID).
			Msg("Successfully sent training task to runner")
	}

	if successCount == 0 {
		return fmt.Errorf("failed to assign any participants to the round")
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

	config := map[string]interface{}{
		"session_id":    session.ID.String(),
		"round_id":      roundID.String(),
		"model_type":    session.ModelType,
		"config":        session.Config,
		"global_model":  session.GlobalModel,
		"training_data": session.TrainingData,
	}

	configData, err := json.Marshal(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal FL task config")
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	task := &models.Task{
		ID:              uuid.New(),
		Title:           fmt.Sprintf("FL Training - Session %s Round %d", session.Name, session.CurrentRound),
		Description:     fmt.Sprintf("Federated learning training task for session %s", session.Name),
		Type:            models.TaskTypeFederatedLearning,
		Status:          models.TaskStatusPending,
		Config:          configData,
		CreatorAddress:  session.CreatorAddress,
		CreatorDeviceID: "fl-coordinator",
		RunnerID:        runnerID,
		Nonce:           fmt.Sprintf("fl-%s-%d", session.ID.String(), session.CurrentRound),
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
		return fmt.Errorf("failed to get participant: %w", err)
	}

	if participant.Status != models.FLParticipantStatusAssigned && participant.Status != models.FLParticipantStatusTraining {
		return fmt.Errorf("participant not in valid status for update submission")
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

	log.Info().Msg("Model update submitted")

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

	aggregatedModel, globalMetrics, err := s.performAggregation(ctx, participants)
	if err != nil {
		return fmt.Errorf("failed to perform aggregation: %w", err)
	}

	aggregation := &models.AggregationResult{
		Method:           "federated_averaging",
		AggregatedModel:  aggregatedModel,
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

func (s *FederatedLearningService) performAggregation(ctx context.Context, participants []*models.FLRoundParticipant) (map[string][]float64, *models.GlobalMetrics, error) {
	if len(participants) == 0 {
		return nil, nil, fmt.Errorf("no participants to aggregate")
	}

	aggregatedModel := make(map[string][]float64)
	totalWeight := 0.0
	totalLoss := 0.0
	totalAccuracy := 0.0
	validParticipants := 0

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

		for layerName, gradients := range modelUpdate.Gradients {
			if _, exists := aggregatedModel[layerName]; !exists {
				aggregatedModel[layerName] = make([]float64, len(gradients))
			}

			for i, gradient := range gradients {
				if i < len(aggregatedModel[layerName]) {
					aggregatedModel[layerName][i] += gradient * weight
				}
			}
		}

		totalLoss += modelUpdate.Loss * weight
		totalAccuracy += modelUpdate.Accuracy * weight
		validParticipants++
	}

	if totalWeight == 0 {
		return nil, nil, fmt.Errorf("total weight is zero")
	}

	for layerName := range aggregatedModel {
		for i := range aggregatedModel[layerName] {
			aggregatedModel[layerName][i] /= totalWeight
		}
	}

	avgLoss := totalLoss / totalWeight
	avgAccuracy := totalAccuracy / totalWeight

	variance := s.calculateVariance(participants, avgLoss)

	globalMetrics := &models.GlobalMetrics{
		AverageLoss:     avgLoss,
		AverageAccuracy: avgAccuracy,
		Variance:        variance,
		Convergence:     s.calculateConvergence(avgLoss, variance),
	}

	return aggregatedModel, globalMetrics, nil
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
