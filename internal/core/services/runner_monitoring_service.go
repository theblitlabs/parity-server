package services

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type MonitoringReportType string

const (
	GoodBehavior       MonitoringReportType = "GoodBehavior"
	PoorPerformance    MonitoringReportType = "PoorPerformance"
	SuspiciousActivity MonitoringReportType = "SuspiciousActivity"
	MaliciousBehavior  MonitoringReportType = "MaliciousBehavior"
	Offline            MonitoringReportType = "Offline"
	ResourceAbuse      MonitoringReportType = "ResourceAbuse"
)

type MonitoringAssignment struct {
	ID          string               `json:"id"`
	MonitorID   string               `json:"monitor_id"`
	TargetID    string               `json:"target_id"`
	StartTime   time.Time            `json:"start_time"`
	Duration    time.Duration        `json:"duration"`
	IsActive    bool                 `json:"is_active"`
	ReportType  MonitoringReportType `json:"report_type,omitempty"`
	Evidence    string               `json:"evidence,omitempty"`
	SubmittedAt *time.Time           `json:"submitted_at,omitempty"`
}

type MonitoringMetrics struct {
	TargetID           string                 `json:"target_id"`
	TasksObserved      int                    `json:"tasks_observed"`
	TasksCompleted     int                    `json:"tasks_completed"`
	TasksFailed        int                    `json:"tasks_failed"`
	AvgResponseTime    time.Duration          `json:"avg_response_time"`
	SuspiciousPatterns []string               `json:"suspicious_patterns"`
	ResourceUsage      map[string]interface{} `json:"resource_usage"`
	LastActivity       time.Time              `json:"last_activity"`
	OfflineDuration    time.Duration          `json:"offline_duration"`
	QualityScore       float64                `json:"quality_score"`
	ReliabilityScore   float64                `json:"reliability_score"`
}

// TaskQueryService interface for getting tasks by runner
type TaskQueryService interface {
	GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error)
}

// ReputationServiceInterface defines the methods we need from reputation service
type ReputationServiceInterface interface {
	ReportMaliciousBehavior(ctx context.Context, runnerID, reason string, evidence map[string]interface{}) error
}

type RunnerMonitoringService struct {
	runnerService     ports.RunnerService
	reputationService ReputationServiceInterface
	taskService       TaskQueryService

	// Monitoring state
	assignments      map[string]*MonitoringAssignment
	metrics          map[string]*MonitoringMetrics
	assignmentsMutex sync.RWMutex
	metricsMutex     sync.RWMutex

	// Configuration
	monitoringInterval   time.Duration
	assignmentDuration   time.Duration
	maxActiveAssignments int

	// Background workers
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewRunnerMonitoringService(
	runnerService ports.RunnerService,
	reputationService ReputationServiceInterface,
	taskService TaskQueryService,
) *RunnerMonitoringService {
	ctx, cancel := context.WithCancel(context.Background())

	return &RunnerMonitoringService{
		runnerService:        runnerService,
		reputationService:    reputationService,
		taskService:          taskService,
		assignments:          make(map[string]*MonitoringAssignment),
		metrics:              make(map[string]*MonitoringMetrics),
		monitoringInterval:   5 * time.Minute,
		assignmentDuration:   24 * time.Hour,
		maxActiveAssignments: 100,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

func (s *RunnerMonitoringService) Start() error {
	log := log.With().Str("component", "runner_monitoring_service").Logger()
	log.Info().Msg("Starting runner monitoring service")

	// Start background workers
	s.wg.Add(3)
	go s.assignmentWorker()
	go s.monitoringWorker()
	go s.reportingWorker()

	return nil
}

func (s *RunnerMonitoringService) Stop() error {
	log := log.With().Str("component", "runner_monitoring_service").Logger()
	log.Info().Msg("Stopping runner monitoring service")

	s.cancel()
	s.wg.Wait()

	log.Info().Msg("Runner monitoring service stopped")
	return nil
}

// Background worker that creates random monitoring assignments
func (s *RunnerMonitoringService) assignmentWorker() {
	defer s.wg.Done()

	log := log.With().Str("worker", "assignment").Logger()
	ticker := time.NewTicker(s.monitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.createRandomAssignments(); err != nil {
				log.Error().Err(err).Msg("Failed to create monitoring assignments")
			}
		}
	}
}

// Background worker that monitors assigned targets
func (s *RunnerMonitoringService) monitoringWorker() {
	defer s.wg.Done()

	log := log.With().Str("worker", "monitoring").Logger()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.updateMonitoringMetrics(); err != nil {
				log.Error().Err(err).Msg("Failed to update monitoring metrics")
			}
		}
	}
}

// Background worker that submits monitoring reports
func (s *RunnerMonitoringService) reportingWorker() {
	defer s.wg.Done()

	log := log.With().Str("worker", "reporting").Logger()
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.submitPendingReports(); err != nil {
				log.Error().Err(err).Msg("Failed to submit monitoring reports")
			}
		}
	}
}

func (s *RunnerMonitoringService) createRandomAssignments() error {
	log := log.With().Str("component", "runner_monitoring_service").Logger()

	// Get all active runners
	activeRunners, err := s.runnerService.ListRunnersByStatus(s.ctx, models.RunnerStatusOnline)
	if err != nil {
		return fmt.Errorf("failed to get active runners: %w", err)
	}

	if len(activeRunners) < 2 {
		log.Debug().Int("runner_count", len(activeRunners)).Msg("Not enough runners for monitoring")
		return nil
	}

	s.assignmentsMutex.Lock()
	defer s.assignmentsMutex.Unlock()

	// Clean up expired assignments
	s.cleanupExpiredAssignments()

	// Check if we can create more assignments
	if len(s.assignments) >= s.maxActiveAssignments {
		log.Debug().Int("active_assignments", len(s.assignments)).Msg("Maximum assignments reached")
		return nil
	}

	// Create random monitoring pairs
	maxNewAssignments := s.maxActiveAssignments - len(s.assignments)
	if maxNewAssignments > len(activeRunners)/2 {
		maxNewAssignments = len(activeRunners) / 2
	}

	// Shuffle runners for randomness
	shuffled := make([]*models.Runner, len(activeRunners))
	copy(shuffled, activeRunners)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	created := 0
	for i := 0; i < len(shuffled)-1 && created < maxNewAssignments; i += 2 {
		monitor := shuffled[i]
		target := shuffled[i+1]

		// Don't assign self-monitoring or duplicate assignments
		if monitor.DeviceID == target.DeviceID {
			continue
		}

		assignmentID := fmt.Sprintf("monitor-%d-%s-%s", time.Now().Unix(), monitor.DeviceID[:8], target.DeviceID[:8])

		assignment := &MonitoringAssignment{
			ID:        assignmentID,
			MonitorID: monitor.DeviceID,
			TargetID:  target.DeviceID,
			StartTime: time.Now(),
			Duration:  s.assignmentDuration,
			IsActive:  true,
		}

		s.assignments[assignmentID] = assignment

		// Initialize metrics for target if not exists
		if _, exists := s.metrics[target.DeviceID]; !exists {
			s.metrics[target.DeviceID] = &MonitoringMetrics{
				TargetID:         target.DeviceID,
				LastActivity:     time.Now(),
				QualityScore:     100.0,
				ReliabilityScore: 100.0,
				ResourceUsage:    make(map[string]interface{}),
			}
		}

		log.Info().
			Str("assignment_id", assignmentID).
			Str("monitor", monitor.DeviceID).
			Str("target", target.DeviceID).
			Msg("Created monitoring assignment")

		created++
	}

	log.Info().Int("assignments_created", created).Msg("Created new monitoring assignments")
	return nil
}

func (s *RunnerMonitoringService) updateMonitoringMetrics() error {
	log := log.With().Str("component", "runner_monitoring_service").Logger()

	s.assignmentsMutex.RLock()
	activeAssignments := make([]*MonitoringAssignment, 0, len(s.assignments))
	for _, assignment := range s.assignments {
		if assignment.IsActive {
			activeAssignments = append(activeAssignments, assignment)
		}
	}
	s.assignmentsMutex.RUnlock()

	for _, assignment := range activeAssignments {
		if err := s.updateTargetMetrics(assignment.TargetID); err != nil {
			log.Error().Err(err).Str("target_id", assignment.TargetID).Msg("Failed to update target metrics")
		}
	}

	return nil
}

func (s *RunnerMonitoringService) updateTargetMetrics(targetID string) error {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	metrics, exists := s.metrics[targetID]
	if !exists {
		metrics = &MonitoringMetrics{
			TargetID:         targetID,
			LastActivity:     time.Now(),
			QualityScore:     100.0,
			ReliabilityScore: 100.0,
			ResourceUsage:    make(map[string]interface{}),
		}
		s.metrics[targetID] = metrics
	}

	// Get runner's recent tasks
	tasks, err := s.taskService.GetTasksByRunner(s.ctx, targetID, 10) // Last 10 tasks
	if err != nil {
		return fmt.Errorf("failed to get tasks for runner %s: %w", targetID, err)
	}

	// Update task metrics
	metrics.TasksObserved = len(tasks)
	completedCount := 0
	failedCount := 0
	totalResponseTime := time.Duration(0)

	for _, task := range tasks {
		if task.Status == models.TaskStatusCompleted {
			completedCount++
			if task.CompletedAt != nil && !task.CreatedAt.IsZero() {
				responseTime := task.CompletedAt.Sub(task.CreatedAt)
				totalResponseTime += responseTime
			}
		} else if task.Status == models.TaskStatusFailed {
			failedCount++
		}

		// Update last activity
		if !task.UpdatedAt.IsZero() && task.UpdatedAt.After(metrics.LastActivity) {
			metrics.LastActivity = task.UpdatedAt
		}
	}

	metrics.TasksCompleted = completedCount
	metrics.TasksFailed = failedCount

	if completedCount > 0 {
		metrics.AvgResponseTime = totalResponseTime / time.Duration(completedCount)
	}

	// Calculate quality and reliability scores
	if metrics.TasksObserved > 0 {
		successRate := float64(completedCount) / float64(metrics.TasksObserved)
		metrics.ReliabilityScore = successRate * 100

		// Quality score based on success rate and response time
		qualityScore := successRate * 100
		if metrics.AvgResponseTime > 0 {
			// Penalize slow responses (longer than 5 minutes)
			if metrics.AvgResponseTime > 5*time.Minute {
				penalty := float64(metrics.AvgResponseTime-5*time.Minute) / float64(time.Minute) * 2
				qualityScore -= penalty
			}
		}
		if qualityScore < 0 {
			qualityScore = 0
		}
		metrics.QualityScore = qualityScore
	}

	// Check for offline status
	if time.Since(metrics.LastActivity) > 30*time.Minute {
		metrics.OfflineDuration = time.Since(metrics.LastActivity)
	} else {
		metrics.OfflineDuration = 0
	}

	// Detect suspicious patterns
	s.detectSuspiciousPatterns(metrics, tasks)

	return nil
}

func (s *RunnerMonitoringService) detectSuspiciousPatterns(metrics *MonitoringMetrics, tasks []*models.Task) {
	patterns := []string{}

	// Pattern 1: High failure rate
	if metrics.TasksObserved >= 5 && float64(metrics.TasksFailed)/float64(metrics.TasksObserved) > 0.8 {
		patterns = append(patterns, "high_failure_rate")
	}

	// Pattern 2: Extremely fast completion (potentially fake)
	if metrics.AvgResponseTime > 0 && metrics.AvgResponseTime < 10*time.Second {
		patterns = append(patterns, "suspiciously_fast_completion")
	}

	// Pattern 3: Regular task abandonment
	abandonedCount := 0
	for _, task := range tasks {
		if task.Status == models.TaskStatusPending {
			abandonedCount++
		}
	}
	if len(tasks) >= 5 && float64(abandonedCount)/float64(len(tasks)) > 0.5 {
		patterns = append(patterns, "frequent_task_abandonment")
	}

	// Pattern 4: Irregular activity patterns
	if metrics.OfflineDuration > 6*time.Hour {
		patterns = append(patterns, "extended_offline_periods")
	}

	metrics.SuspiciousPatterns = patterns
}

func (s *RunnerMonitoringService) submitPendingReports() error {
	log := log.With().Str("component", "runner_monitoring_service").Logger()

	s.assignmentsMutex.Lock()
	defer s.assignmentsMutex.Unlock()

	now := time.Now()

	for assignmentID, assignment := range s.assignments {
		if !assignment.IsActive || assignment.SubmittedAt != nil {
			continue
		}

		// Check if assignment is ready for reporting (halfway through or completed)
		elapsed := now.Sub(assignment.StartTime)
		if elapsed < assignment.Duration/2 && elapsed < assignment.Duration {
			continue
		}

		// Generate monitoring report
		reportType, evidence := s.generateMonitoringReport(assignment.TargetID)

		// Submit report to reputation system
		if err := s.submitMonitoringReport(assignmentID, assignment.MonitorID, assignment.TargetID, reportType, evidence); err != nil {
			log.Error().Err(err).
				Str("assignment_id", assignmentID).
				Str("monitor", assignment.MonitorID).
				Str("target", assignment.TargetID).
				Msg("Failed to submit monitoring report")
			continue
		}

		// Mark as submitted
		assignment.ReportType = reportType
		assignment.Evidence = evidence
		submittedAt := now
		assignment.SubmittedAt = &submittedAt
		assignment.IsActive = false

		log.Info().
			Str("assignment_id", assignmentID).
			Str("monitor", assignment.MonitorID).
			Str("target", assignment.TargetID).
			Str("report_type", string(reportType)).
			Msg("Submitted monitoring report")
	}

	return nil
}

func (s *RunnerMonitoringService) generateMonitoringReport(targetID string) (MonitoringReportType, string) {
	s.metricsMutex.RLock()
	metrics, exists := s.metrics[targetID]
	s.metricsMutex.RUnlock()

	if !exists {
		return Offline, "No metrics available for target runner"
	}

	// Determine report type based on metrics
	if metrics.OfflineDuration > 12*time.Hour {
		return Offline, fmt.Sprintf("Runner offline for %v", metrics.OfflineDuration)
	}

	// Check for malicious patterns
	for _, pattern := range metrics.SuspiciousPatterns {
		switch pattern {
		case "high_failure_rate":
			return MaliciousBehavior, fmt.Sprintf("High failure rate: %d/%d tasks failed", metrics.TasksFailed, metrics.TasksObserved)
		case "suspiciously_fast_completion":
			return SuspiciousActivity, fmt.Sprintf("Suspiciously fast task completion: avg %v", metrics.AvgResponseTime)
		case "frequent_task_abandonment":
			return PoorPerformance, "Frequent task abandonment detected"
		}
	}

	// Quality-based assessment
	if metrics.QualityScore < 30 {
		return PoorPerformance, fmt.Sprintf("Low quality score: %.1f", metrics.QualityScore)
	} else if metrics.ReliabilityScore < 50 {
		return PoorPerformance, fmt.Sprintf("Low reliability score: %.1f", metrics.ReliabilityScore)
	}

	// Default to good behavior
	return GoodBehavior, fmt.Sprintf("Quality: %.1f, Reliability: %.1f", metrics.QualityScore, metrics.ReliabilityScore)
}

func (s *RunnerMonitoringService) submitMonitoringReport(assignmentID, monitorID, targetID string, reportType MonitoringReportType, evidence string) error {
	// Create evidence structure
	evidenceData := map[string]interface{}{
		"assignment_id": assignmentID,
		"monitor_id":    monitorID,
		"target_id":     targetID,
		"report_type":   reportType,
		"evidence":      evidence,
		"timestamp":     time.Now().Unix(),
	}

	s.metricsMutex.RLock()
	if metrics, exists := s.metrics[targetID]; exists {
		evidenceData["metrics"] = metrics
	}
	s.metricsMutex.RUnlock()

	// Convert to reputation event type
	var scoreDelta int

	switch reportType {
	case GoodBehavior:
		scoreDelta = 5
	case PoorPerformance:
		scoreDelta = -10
	case SuspiciousActivity:
		scoreDelta = -20
	case MaliciousBehavior:
		scoreDelta = -50
	case Offline:
		scoreDelta = -5
	case ResourceAbuse:
		scoreDelta = -30
	default:
		scoreDelta = 0
	}

	// Report malicious behavior for severe cases
	if reportType == MaliciousBehavior {
		return s.reputationService.ReportMaliciousBehavior(s.ctx, targetID, evidence, evidenceData)
	}

	// For other cases, we would need to implement a general reputation update method
	// For now, just log the report
	log.Info().
		Str("assignment_id", assignmentID).
		Str("monitor_id", monitorID).
		Str("target_id", targetID).
		Str("report_type", string(reportType)).
		Int("score_delta", scoreDelta).
		Str("evidence", evidence).
		Msg("Monitoring report submitted")

	return nil
}

func (s *RunnerMonitoringService) cleanupExpiredAssignments() {
	now := time.Now()

	for assignmentID, assignment := range s.assignments {
		if now.Sub(assignment.StartTime) > assignment.Duration {
			delete(s.assignments, assignmentID)
		}
	}
}

// Public methods for querying monitoring data

func (s *RunnerMonitoringService) GetActiveAssignments() []*MonitoringAssignment {
	s.assignmentsMutex.RLock()
	defer s.assignmentsMutex.RUnlock()

	assignments := make([]*MonitoringAssignment, 0, len(s.assignments))
	for _, assignment := range s.assignments {
		if assignment.IsActive {
			assignments = append(assignments, assignment)
		}
	}

	return assignments
}

func (s *RunnerMonitoringService) GetRunnerMetrics(runnerID string) *MonitoringMetrics {
	s.metricsMutex.RLock()
	defer s.metricsMutex.RUnlock()

	if metrics, exists := s.metrics[runnerID]; exists {
		return metrics
	}

	return nil
}

func (s *RunnerMonitoringService) GetMonitoringStats() map[string]interface{} {
	s.assignmentsMutex.RLock()
	s.metricsMutex.RLock()
	defer s.assignmentsMutex.RUnlock()
	defer s.metricsMutex.RUnlock()

	activeCount := 0
	for _, assignment := range s.assignments {
		if assignment.IsActive {
			activeCount++
		}
	}

	return map[string]interface{}{
		"active_assignments":  activeCount,
		"total_assignments":   len(s.assignments),
		"monitored_runners":   len(s.metrics),
		"monitoring_interval": s.monitoringInterval.String(),
	}
}
