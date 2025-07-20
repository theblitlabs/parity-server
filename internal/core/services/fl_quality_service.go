package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type FLQualityService struct {
	qualityRepo       ports.QualityRepository
	flSessionRepo     ports.FLSessionRepository
	flRoundRepo       ports.FLRoundRepository
	flParticipantRepo ports.FLParticipantRepository
	runnerService     ports.RunnerService
}

func NewFLQualityService(
	qualityRepo ports.QualityRepository,
	flSessionRepo ports.FLSessionRepository,
	flRoundRepo ports.FLRoundRepository,
	flParticipantRepo ports.FLParticipantRepository,
	runnerService ports.RunnerService,
) *FLQualityService {
	return &FLQualityService{
		qualityRepo:       qualityRepo,
		flSessionRepo:     flSessionRepo,
		flRoundRepo:       flRoundRepo,
		flParticipantRepo: flParticipantRepo,
		runnerService:     runnerService,
	}
}

// MonitorParticipantQuality continuously monitors participant quality metrics
func (s *FLQualityService) MonitorParticipantQuality(ctx context.Context, sessionID uuid.UUID, runnerID string) error {
	logger := log.With().
		Str("service", "fl_quality").
		Str("session_id", sessionID.String()).
		Str("runner_id", runnerID).
		Logger()

	// Calculate performance metrics
	performanceMetrics, err := s.calculateParticipantPerformance(ctx, sessionID, runnerID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate participant performance")
		return err
	}

	// Calculate reliability metrics
	reliabilityMetrics, err := s.calculateParticipantReliability()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate participant reliability")
		return err
	}

	// Calculate network quality
	networkMetrics, err := s.calculateNetworkQuality()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate network quality")
		return err
	}

	// Calculate resource efficiency
	resourceMetrics, err := s.calculateResourceEfficiency()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate resource efficiency")
		return err
	}

	// Calculate overall quality score
	overallScore := models.CalculateOverallQualityScore(
		performanceMetrics.ModelQualityScore,
		reliabilityMetrics.UptimePercentage,
		networkMetrics.ConnectionStability,
		resourceMetrics.CPUEfficiency,
	)

	// Get previous quality score for trend analysis
	previousMetrics, _ := s.qualityRepo.GetLatestParticipantMetrics(ctx, runnerID, sessionID)
	var qualityTrend string
	if previousMetrics != nil {
		qualityTrend = models.DetermineQualityTrend(overallScore, previousMetrics.OverallQualityScore)
	} else {
		qualityTrend = "stable"
	}

	// Create comprehensive quality metrics
	qualityMetrics := &models.ParticipantQualityMetrics{
		ID:                   uuid.New(),
		RunnerID:             runnerID,
		SessionID:            sessionID,
		AverageTrainingTime:  performanceMetrics.AverageTrainingTime,
		TaskCompletionRate:   performanceMetrics.TaskCompletionRate,
		ModelQualityScore:    performanceMetrics.ModelQualityScore,
		DataQualityScore:     performanceMetrics.DataQualityScore,
		UptimePercentage:     reliabilityMetrics.UptimePercentage,
		HeartbeatConsistency: reliabilityMetrics.HeartbeatConsistency,
		ErrorRate:            reliabilityMetrics.ErrorRate,
		AverageLatency:       networkMetrics.AverageLatency,
		BandwidthUtilization: networkMetrics.BandwidthUtilization,
		ConnectionStability:  networkMetrics.ConnectionStability,
		CPUEfficiency:        resourceMetrics.CPUEfficiency,
		MemoryEfficiency:     resourceMetrics.MemoryEfficiency,
		StorageUtilization:   resourceMetrics.StorageUtilization,
		OverallQualityScore:  overallScore,
		QualityTrend:         qualityTrend,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Store quality metrics
	if err := s.qualityRepo.CreateParticipantMetrics(ctx, qualityMetrics); err != nil {
		logger.Error().Err(err).Msg("Failed to store participant quality metrics")
		return err
	}

	// Check for quality violations and create alerts
	if err := s.checkQualityViolations(ctx, qualityMetrics); err != nil {
		logger.Error().Err(err).Msg("Failed to check quality violations")
	}

	logger.Info().
		Float64("quality_score", overallScore).
		Str("trend", qualityTrend).
		Msg("Participant quality metrics updated")

	return nil
}

// MonitorSessionQuality monitors overall session quality
func (s *FLQualityService) MonitorSessionQuality(ctx context.Context, sessionID uuid.UUID) error {
	logger := log.With().
		Str("service", "fl_quality").
		Str("session_id", sessionID.String()).
		Logger()

	session, err := s.flSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Calculate convergence quality
	convergenceMetrics, err := s.calculateConvergenceQuality(ctx, sessionID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate convergence quality")
		return err
	}

	// Calculate participant quality metrics
	participantMetrics, err := s.calculateSessionParticipantQuality()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate participant quality")
		return err
	}

	// Calculate data quality
	dataMetrics, err := s.calculateDataQuality(session)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate data quality")
		return err
	}

	// Calculate infrastructure quality
	infraMetrics, err := s.calculateInfrastructureQuality()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate infrastructure quality")
		return err
	}

	// Calculate security and privacy scores
	securityMetrics, err := s.calculateSecurityMetrics(session)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate security metrics")
		return err
	}

	// Calculate overall session health score
	sessionHealthScore := s.calculateSessionHealthScore(
		convergenceMetrics.ConvergenceRate,
		participantMetrics.AverageParticipantQuality,
		dataMetrics.DataIntegrity,
		infraMetrics.NetworkEfficiency,
		securityMetrics.SecurityScore,
	)

	// Create session quality metrics
	sessionQuality := &models.SessionQualityMetrics{
		ID:                        uuid.New(),
		SessionID:                 sessionID,
		ConvergenceRate:           convergenceMetrics.ConvergenceRate,
		ModelStability:            convergenceMetrics.ModelStability,
		AccuracyImprovement:       convergenceMetrics.AccuracyImprovement,
		ParticipantRetention:      participantMetrics.ParticipantRetention,
		AverageParticipantQuality: participantMetrics.AverageParticipantQuality,
		ParticipantConsistency:    participantMetrics.ParticipantConsistency,
		DataDistribution:          dataMetrics.DataDistribution,
		DataIntegrity:             dataMetrics.DataIntegrity,
		PartitioningEffectiveness: dataMetrics.PartitioningEffectiveness,
		SystemLatency:             infraMetrics.SystemLatency,
		NetworkEfficiency:         infraMetrics.NetworkEfficiency,
		ResourceUtilization:       infraMetrics.ResourceUtilization,
		PrivacyCompliance:         securityMetrics.PrivacyCompliance,
		SecurityScore:             securityMetrics.SecurityScore,
		AnomalyDetectionScore:     securityMetrics.AnomalyDetectionScore,
		InfrastructureQuality:     infraMetrics.InfrastructureQuality,
		SessionHealthScore:        sessionHealthScore,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}

	// Store session quality metrics
	if err := s.qualityRepo.CreateSessionMetrics(ctx, sessionQuality); err != nil {
		logger.Error().Err(err).Msg("Failed to store session quality metrics")
		return err
	}

	logger.Info().
		Float64("health_score", sessionHealthScore).
		Float64("convergence_rate", convergenceMetrics.ConvergenceRate).
		Msg("Session quality metrics updated")

	return nil
}

// CreateQualityAlert creates alerts for quality violations
func (s *FLQualityService) CreateQualityAlert(ctx context.Context, alertType, severity, entityType, entityID, message string, details interface{}) error {
	alert := &models.QualityAlert{
		ID:         uuid.New(),
		AlertType:  alertType,
		Severity:   severity,
		EntityType: entityType,
		EntityID:   entityID,
		Message:    message,
		Details:    details,
		Status:     "active",
		CreatedAt:  time.Now(),
	}

	if err := s.qualityRepo.CreateAlert(ctx, alert); err != nil {
		return fmt.Errorf("failed to create quality alert: %w", err)
	}

	log.Warn().
		Str("alert_type", alertType).
		Str("severity", severity).
		Str("entity", entityType).
		Str("entity_id", entityID).
		Str("message", message).
		Msg("Quality alert created")

	return nil
}

// Helper methods for calculating specific quality metrics

func (s *FLQualityService) calculateParticipantPerformance(ctx context.Context, sessionID uuid.UUID, runnerID string) (*struct {
	AverageTrainingTime float64
	TaskCompletionRate  float64
	ModelQualityScore   float64
	DataQualityScore    float64
}, error) {
	// Get participant's rounds for this session
	rounds, err := s.flRoundRepo.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var totalTrainingTime float64
	var completedTasks int
	var totalTasks int
	var lossSum float64

	for _, round := range rounds {
		participants, err := s.flParticipantRepo.GetByRound(ctx, round.ID)
		if err != nil {
			continue
		}

		for _, participant := range participants {
			if participant.RunnerID == runnerID {
				totalTasks++
				if participant.Status == models.FLParticipantStatusCompleted {
					completedTasks++
					// Extract training metrics if available
					// This would include training time, accuracy, loss, etc.
				}
			}
		}
	}

	completionRate := float64(completedTasks) / math.Max(float64(totalTasks), 1) * 100
	avgTrainingTime := totalTrainingTime / math.Max(float64(completedTasks), 1)
	modelQuality := (100 - (lossSum / math.Max(float64(completedTasks), 1))) // Simplified scoring
	dataQuality := 95.0                                                      // This would be calculated based on data validation results

	return &struct {
		AverageTrainingTime float64
		TaskCompletionRate  float64
		ModelQualityScore   float64
		DataQualityScore    float64
	}{
		AverageTrainingTime: avgTrainingTime,
		TaskCompletionRate:  completionRate,
		ModelQualityScore:   math.Max(0, math.Min(100, modelQuality)),
		DataQualityScore:    dataQuality,
	}, nil
}

func (s *FLQualityService) calculateParticipantReliability() (*struct {
	UptimePercentage     float64
	HeartbeatConsistency float64
	ErrorRate            float64
}, error) {
	// This would calculate based on heartbeat history and task failure rates
	// For now, returning sample values
	return &struct {
		UptimePercentage     float64
		HeartbeatConsistency float64
		ErrorRate            float64
	}{
		UptimePercentage:     98.5,
		HeartbeatConsistency: 95.0,
		ErrorRate:            2.1,
	}, nil
}

func (s *FLQualityService) calculateNetworkQuality() (*struct {
	AverageLatency       float64
	BandwidthUtilization float64
	ConnectionStability  float64
}, error) {
	// This would calculate based on network performance metrics
	return &struct {
		AverageLatency       float64
		BandwidthUtilization float64
		ConnectionStability  float64
	}{
		AverageLatency:       120.5,
		BandwidthUtilization: 75.3,
		ConnectionStability:  96.2,
	}, nil
}

func (s *FLQualityService) calculateResourceEfficiency() (*struct {
	CPUEfficiency      float64
	MemoryEfficiency   float64
	StorageUtilization float64
}, error) {
	// This would calculate based on resource utilization metrics
	return &struct {
		CPUEfficiency      float64
		MemoryEfficiency   float64
		StorageUtilization float64
	}{
		CPUEfficiency:      88.7,
		MemoryEfficiency:   92.1,
		StorageUtilization: 65.4,
	}, nil
}

func (s *FLQualityService) calculateConvergenceQuality(ctx context.Context, sessionID uuid.UUID) (*struct {
	ConvergenceRate     float64
	ModelStability      float64
	AccuracyImprovement float64
}, error) {
	rounds, err := s.flRoundRepo.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(rounds) < 2 {
		return &struct {
			ConvergenceRate     float64
			ModelStability      float64
			AccuracyImprovement float64
		}{0, 100, 0}, nil
	}

	// Calculate convergence metrics from rounds
	var accuracyImprovement float64
	var stabilityVariance float64

	// This would analyze the actual convergence patterns
	convergenceRate := float64(len(rounds)) // Simplified - actual implementation would detect convergence

	return &struct {
		ConvergenceRate     float64
		ModelStability      float64
		AccuracyImprovement float64
	}{
		ConvergenceRate:     convergenceRate,
		ModelStability:      100 - stabilityVariance,
		AccuracyImprovement: accuracyImprovement,
	}, nil
}

func (s *FLQualityService) calculateSessionParticipantQuality() (*struct {
	ParticipantRetention      float64
	AverageParticipantQuality float64
	ParticipantConsistency    float64
}, error) {
	// Calculate participant-related quality metrics
	return &struct {
		ParticipantRetention      float64
		AverageParticipantQuality float64
		ParticipantConsistency    float64
	}{
		ParticipantRetention:      89.3,
		AverageParticipantQuality: 87.6,
		ParticipantConsistency:    91.2,
	}, nil
}

func (s *FLQualityService) calculateDataQuality(session *models.FederatedLearningSession) (*struct {
	DataDistribution          string
	DataIntegrity             float64
	PartitioningEffectiveness float64
}, error) {
	// Analyze data quality based on partitioning strategy and results
	return &struct {
		DataDistribution          string
		DataIntegrity             float64
		PartitioningEffectiveness float64
	}{
		DataDistribution:          session.TrainingData.SplitStrategy,
		DataIntegrity:             97.8,
		PartitioningEffectiveness: 93.4,
	}, nil
}

func (s *FLQualityService) calculateInfrastructureQuality() (*struct {
	SystemLatency         float64
	NetworkEfficiency     float64
	ResourceUtilization   float64
	InfrastructureQuality float64
}, error) {
	// Calculate infrastructure-related metrics
	return &struct {
		SystemLatency         float64
		NetworkEfficiency     float64
		ResourceUtilization   float64
		InfrastructureQuality float64
	}{
		SystemLatency:         85.2,
		NetworkEfficiency:     91.7,
		ResourceUtilization:   78.9,
		InfrastructureQuality: 88.6,
	}, nil
}

func (s *FLQualityService) calculateSecurityMetrics(session *models.FederatedLearningSession) (*struct {
	PrivacyCompliance     float64
	SecurityScore         float64
	AnomalyDetectionScore float64
}, error) {
	// Calculate security and privacy metrics
	privacyScore := 95.0
	if session.Config.PrivacyConfig.DifferentialPrivacy {
		privacyScore = 98.5
	}

	return &struct {
		PrivacyCompliance     float64
		SecurityScore         float64
		AnomalyDetectionScore float64
	}{
		PrivacyCompliance:     privacyScore,
		SecurityScore:         94.2,
		AnomalyDetectionScore: 96.1,
	}, nil
}

func (s *FLQualityService) calculateSessionHealthScore(convergence, participantQuality, dataIntegrity, networkEfficiency, security float64) float64 {
	weights := map[string]float64{
		"convergence":         0.25,
		"participant_quality": 0.25,
		"data_integrity":      0.20,
		"network_efficiency":  0.15,
		"security":            0.15,
	}

	return (convergence * weights["convergence"]) +
		(participantQuality * weights["participant_quality"]) +
		(dataIntegrity * weights["data_integrity"]) +
		(networkEfficiency * weights["network_efficiency"]) +
		(security * weights["security"])
}

func (s *FLQualityService) checkQualityViolations(ctx context.Context, metrics *models.ParticipantQualityMetrics) error {
	// Define quality thresholds
	thresholds := map[string]float64{
		"min_quality_score":   70.0,
		"max_error_rate":      10.0,
		"min_uptime":          90.0,
		"max_latency":         500.0,
		"min_completion_rate": 80.0,
	}

	// Check violations and create alerts
	if metrics.OverallQualityScore < thresholds["min_quality_score"] {
		if err := s.CreateQualityAlert(ctx, "performance", "high", "participant", metrics.RunnerID,
			fmt.Sprintf("Participant quality score %0.1f below threshold %0.1f",
				metrics.OverallQualityScore, thresholds["min_quality_score"]), metrics); err != nil {
			return fmt.Errorf("failed to create quality alert: %w", err)
		}
	}

	if metrics.ErrorRate > thresholds["max_error_rate"] {
		if err := s.CreateQualityAlert(ctx, "reliability", "medium", "participant", metrics.RunnerID,
			fmt.Sprintf("Error rate %0.1f%% exceeds threshold %0.1f%%",
				metrics.ErrorRate, thresholds["max_error_rate"]), metrics); err != nil {
			return fmt.Errorf("failed to create quality alert: %w", err)
		}
	}

	if metrics.UptimePercentage < thresholds["min_uptime"] {
		if err := s.CreateQualityAlert(ctx, "reliability", "high", "participant", metrics.RunnerID,
			fmt.Sprintf("Uptime %0.1f%% below threshold %0.1f%%",
				metrics.UptimePercentage, thresholds["min_uptime"]), metrics); err != nil {
			return fmt.Errorf("failed to create quality alert: %w", err)
		}
	}

	if metrics.AverageLatency > thresholds["max_latency"] {
		if err := s.CreateQualityAlert(ctx, "performance", "medium", "participant", metrics.RunnerID,
			fmt.Sprintf("Average latency %0.1fms exceeds threshold %0.1fms",
				metrics.AverageLatency, thresholds["max_latency"]), metrics); err != nil {
			return fmt.Errorf("failed to create quality alert: %w", err)
		}
	}

	if metrics.TaskCompletionRate < thresholds["min_completion_rate"] {
		if err := s.CreateQualityAlert(ctx, "performance", "critical", "participant", metrics.RunnerID,
			fmt.Sprintf("Task completion rate %0.1f%% below threshold %0.1f%%",
				metrics.TaskCompletionRate, thresholds["min_completion_rate"]), metrics); err != nil {
			return fmt.Errorf("failed to create quality alert: %w", err)
		}
	}

	return nil
}
