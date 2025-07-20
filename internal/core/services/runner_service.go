package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

var ErrRunnerNotFound = errors.New("runner not found")

type RunnerRepository interface {
	Create(ctx context.Context, runner *models.Runner) error
	Get(ctx context.Context, deviceID string) (*models.Runner, error)
	CreateOrUpdate(ctx context.Context, runner *models.Runner) (*models.Runner, error)
	Update(ctx context.Context, runner *models.Runner) (*models.Runner, error)
	ListByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error)
	UpdateRunnersToOffline(ctx context.Context, heartbeatTimeout time.Duration) (int64, []string, error)
	GetOnlineRunners(ctx context.Context) ([]*models.Runner, error)
	GetRunnerByDeviceID(ctx context.Context, deviceID string) (*models.Runner, error)
	UpdateModelCapabilities(ctx context.Context, runnerID string, capabilities []models.ModelCapability) error
}

type RunnerService struct {
	repo             RunnerRepository
	taskService      *TaskService
	heartbeatTimeout time.Duration
	taskMonitorCh    chan struct{}
}

func NewRunnerService(repo RunnerRepository) *RunnerService {
	return &RunnerService{
		repo:             repo,
		heartbeatTimeout: 2 * time.Minute,
		taskMonitorCh:    make(chan struct{}, 10),
	}
}

func (s *RunnerService) SetTaskService(taskService *TaskService) {
	s.taskService = taskService

	go s.taskMonitorWorker()
}

func (s *RunnerService) taskMonitorWorker() {
	var timer *time.Timer
	for range s.taskMonitorCh {
		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(100*time.Millisecond, func() {
			if s.taskService != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				runners, err := s.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
				if err != nil {
					log := gologger.WithComponent("runner_service")
					log.Error().Err(err).Msg("Failed to list online runners")
					return
				}

				for _, runner := range runners {
					if err := s.taskService.checkAndAssignPendingTasksToRunner(ctx, runner.DeviceID); err != nil {
						log := gologger.WithComponent("runner_service")
						log.Error().Err(err).
							Str("runner_id", runner.DeviceID).
							Msg("Failed to check and assign pending tasks to runner")
					}
				}
			}
		})
	}
}

func (s *RunnerService) triggerTaskMonitor() {
	select {
	case s.taskMonitorCh <- struct{}{}:
	default:
	}
}

func (s *RunnerService) SetHeartbeatTimeout(timeout time.Duration) {
	s.heartbeatTimeout = timeout
}

func (s *RunnerService) CreateRunner(ctx context.Context, runner *models.Runner) error {
	return s.repo.Create(ctx, runner)
}

func (s *RunnerService) GetRunner(ctx context.Context, deviceID string) (*models.Runner, error) {
	return s.repo.Get(ctx, deviceID)
}

func (s *RunnerService) CreateOrUpdateRunner(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	existingRunner, err := s.repo.Get(ctx, runner.DeviceID)

	isNewOrBecomingAvailable := false

	if err != nil {
		isNewOrBecomingAvailable = runner.Status == models.RunnerStatusOnline
	} else {
		isNewOrBecomingAvailable = (existingRunner.Status == models.RunnerStatusOffline ||
			existingRunner.Status == models.RunnerStatusBusy) &&
			runner.Status == models.RunnerStatusOnline
	}

	updatedRunner, err := s.repo.CreateOrUpdate(ctx, runner)
	if err != nil {
		return nil, err
	}

	if isNewOrBecomingAvailable {
		if s.taskService != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := s.taskService.checkAndAssignPendingTasksToRunner(ctx, runner.DeviceID); err != nil {
					log := gologger.WithComponent("runner_service")
					log.Error().Err(err).
						Str("runner_id", runner.DeviceID).
						Msg("Failed to assign pending tasks to newly available runner")
				}
			}()
		}
	}

	return updatedRunner, nil
}

func (s *RunnerService) UpdateRunner(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	return s.repo.Update(ctx, runner)
}

func (s *RunnerService) ListRunnersByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	return s.repo.ListByStatus(ctx, status)
}

func (s *RunnerService) UpdateRunnerStatus(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	existingRunner, err := s.repo.Get(ctx, runner.DeviceID)
	if err != nil {
		return nil, err
	}

	becomingAvailable := (existingRunner.Status == models.RunnerStatusOffline ||
		existingRunner.Status == models.RunnerStatusBusy) &&
		runner.Status == models.RunnerStatusOnline

	existingRunner.Status = runner.Status

	if runner.WalletAddress != "" {
		existingRunner.WalletAddress = runner.WalletAddress
	}

	if runner.Status == models.RunnerStatusOnline {
		log := gologger.WithComponent("runner_service")
		log.Debug().
			Str("device_id", runner.DeviceID).
			Msg("Updating runner heartbeat")
	}

	if runner.Webhook != "" {
		existingRunner.Webhook = runner.Webhook
	}

	updatedRunner, err := s.repo.Update(ctx, existingRunner)
	if err != nil {
		return nil, err
	}

	if becomingAvailable {
		s.triggerTaskMonitor()
	}

	return updatedRunner, nil
}

func (s *RunnerService) ForwardPromptToRunner(ctx context.Context, runnerID string, promptReq *models.PromptRequest) error {
	log := gologger.WithComponent("runner_service")

	runner, err := s.repo.Get(ctx, runnerID)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to get runner")
		return fmt.Errorf("failed to get runner: %w", err)
	}

	if runner.Webhook == "" {
		log.Error().Str("runner_id", runnerID).Msg("Runner has no webhook URL")
		return fmt.Errorf("runner %s has no webhook URL", runnerID)
	}

	// Create LLM task config in the format expected by the runner executor
	configData, err := json.Marshal(map[string]interface{}{
		"model":  promptReq.ModelName,
		"prompt": promptReq.Prompt,
	})
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to marshal task config")
		return fmt.Errorf("failed to marshal task config: %w", err)
	}

	task := &models.Task{
		ID:          promptReq.ID,
		Title:       fmt.Sprintf("LLM Prompt: %s", promptReq.ModelName),
		Description: fmt.Sprintf("Generate response for prompt using model %s", promptReq.ModelName),
		Type:        models.TaskTypeLLM,
		Config:      configData,
		Environment: &models.EnvironmentConfig{
			Type: "llm",
			Config: map[string]interface{}{
				"MODEL":  promptReq.ModelName,
				"PROMPT": promptReq.Prompt,
			},
		},
		CreatorAddress:  promptReq.CreatorAddress,
		CreatorDeviceID: "server",
		RunnerID:        runnerID,
		Reward:          0.0,
		Nonce:           hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))),
		Status:          models.TaskStatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Store the task in the database so the runner can find it
	if s.taskService != nil {
		if err := s.taskService.CreateTask(ctx, task); err != nil {
			log.Error().Err(err).Str("runner_id", runnerID).Str("task_id", task.ID.String()).Msg("Failed to store task in database")
			return fmt.Errorf("failed to store task in database: %w", err)
		}
		log.Info().Str("task_id", task.ID.String()).Msg("Task stored in database successfully")

		// Assign the task to the runner
		if err := s.taskService.AssignTaskToRunner(ctx, task.ID.String(), runnerID); err != nil {
			log.Error().Err(err).Str("runner_id", runnerID).Str("task_id", task.ID.String()).Msg("Failed to assign task to runner")
			// Mark the task as failed if assignment fails
			if failErr := s.taskService.FailTask(ctx, task.ID.String(), "Failed to assign task to runner"); failErr != nil {
				log.Error().Err(failErr).Str("task_id", task.ID.String()).Msg("Failed to mark task as failed after assignment failure")
			}
			return fmt.Errorf("failed to assign task to runner: %w", err)
		}
	} else {
		log.Warn().Msg("TaskService not available, task will not be stored in database")
		return fmt.Errorf("taskService not available")
	}

	// Create webhook message
	type WebhookMessage struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	taskPayload, err := json.Marshal(task)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to marshal task payload")
		s.cleanupFailedTask(ctx, task.ID.String(), runnerID, "Failed to marshal task payload")
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	message := WebhookMessage{
		Type:    "available_tasks",
		Payload: taskPayload,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to marshal webhook message")
		s.cleanupFailedTask(ctx, task.ID.String(), runnerID, "Failed to marshal webhook message")
		return fmt.Errorf("failed to marshal webhook message: %w", err)
	}

	log.Info().
		Str("runner_id", runnerID).
		Str("prompt_id", promptReq.ID.String()).
		Str("webhook", runner.Webhook).
		Msg("Forwarding prompt to runner")

	// Send HTTP request to runner webhook (with longer timeout)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", runner.Webhook, bytes.NewBuffer(messageBytes))
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to create HTTP request")
		s.cleanupFailedTask(ctx, task.ID.String(), runnerID, "Failed to create HTTP request")
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second, // Webhook should respond immediately
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Str("webhook", runner.Webhook).Msg("Failed to send request to runner webhook")
		s.cleanupFailedTask(ctx, task.ID.String(), runnerID, fmt.Sprintf("Webhook delivery failed: %v", err))
		return fmt.Errorf("failed to send request to runner webhook: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("runner_id", runnerID).
			Str("webhook", runner.Webhook).
			Msg("Runner webhook returned non-OK status")
		s.cleanupFailedTask(ctx, task.ID.String(), runnerID, fmt.Sprintf("Webhook returned status %d", resp.StatusCode))
		return fmt.Errorf("runner webhook returned status %d", resp.StatusCode)
	}

	log.Info().
		Str("runner_id", runnerID).
		Str("prompt_id", promptReq.ID.String()).
		Int("status_code", resp.StatusCode).
		Msg("Prompt forwarded to runner successfully")

	return nil
}

// cleanupFailedTask cleans up resources when task delivery fails
func (s *RunnerService) cleanupFailedTask(ctx context.Context, taskID, runnerID, reason string) {
	log := gologger.WithComponent("runner_service")

	log.Info().
		Str("task_id", taskID).
		Str("runner_id", runnerID).
		Str("reason", reason).
		Msg("Cleaning up failed task")

	if s.taskService != nil {
		// FailTask will mark task as failed and clear runner assignment
		if err := s.taskService.FailTask(ctx, taskID, reason); err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("Failed to mark task as failed during cleanup")
		} else {
			log.Info().Str("task_id", taskID).Str("runner_id", runnerID).Msg("Task marked as failed and runner freed")
		}
	}
}

func (s *RunnerService) UpdateModelCapabilities(ctx context.Context, runnerID string, capabilities []models.ModelCapability) error {
	if repo, ok := s.repo.(interface {
		UpdateModelCapabilities(ctx context.Context, runnerID string, capabilities []models.ModelCapability) error
	}); ok {
		return repo.UpdateModelCapabilities(ctx, runnerID, capabilities)
	}
	return fmt.Errorf("repository does not support model capabilities")
}

func (s *RunnerService) UpdateOfflineRunners(ctx context.Context) error {
	log := gologger.WithComponent("runner_service")

	count, deviceIDs, err := s.repo.UpdateRunnersToOffline(ctx, s.heartbeatTimeout)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update offline runners")
		return err
	}

	if count > 0 {
		var deviceIDsStr string
		maxDisplay := 3

		if len(deviceIDs) <= maxDisplay {
			deviceIDsStr = strings.Join(deviceIDs, ", ")
		} else {
			deviceIDsStr = fmt.Sprintf("%s and %d more",
				strings.Join(deviceIDs[:maxDisplay], ", "),
				len(deviceIDs)-maxDisplay)
		}

		log.Info().
			Int64("count", count).
			Dur("timeout", s.heartbeatTimeout).
			Str("runner_ids", deviceIDsStr).
			Msg("Marked runners as offline due to heartbeat timeout")

		s.triggerTaskMonitor()
	} else {
		log.Debug().
			Dur("timeout", s.heartbeatTimeout).
			Msg("Heartbeat check completed - all runners active")
	}

	return nil
}

func (s *RunnerService) StopTaskMonitor() error {
	if s.taskMonitorCh != nil {
		close(s.taskMonitorCh)
	}
	return nil
}

func (s *RunnerService) GetAvailableRunnerForModel(ctx context.Context, modelName string) (string, error) {
	runners, err := s.repo.GetOnlineRunners(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get online runners: %w", err)
	}

	for _, runner := range runners {
		// Check if runner is actually available (no task assigned)
		if runner.TaskID != nil {
			continue // Runner is busy with another task
		}

		// Check if runner has the required model capability
		for _, capability := range runner.ModelCapabilities {
			if capability.ModelName == modelName && capability.IsLoaded {
				if runner.Status == models.RunnerStatusOnline {
					return runner.DeviceID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no available runner found for model %s", modelName)
}
