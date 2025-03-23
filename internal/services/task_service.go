package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
)

var (
	ErrInvalidTask  = errors.New("invalid task data")
	ErrTaskNotFound = repositories.ErrTaskNotFound
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	List(ctx context.Context, limit, offset int) ([]*models.Task, error)
	ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error)
	GetAll(ctx context.Context) ([]models.Task, error)
	SaveTaskResult(ctx context.Context, result *models.TaskResult) error
	GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error)
}

// RewardCalculatorService interface is now defined in ports.RewardCalculator

type TaskService struct {
	repo                   TaskRepository
	rewardCalculator       ports.RewardCalculator
	rewardClient           ports.RewardClient
	nonceService           *NonceService
	runnerService          *RunnerService
	notificationInProgress sync.Map // Used to track in-progress notifications
}

func NewTaskService(repo TaskRepository, rewardCalculator ports.RewardCalculator, runnerService *RunnerService) *TaskService {
	return &TaskService{
		repo:             repo,
		rewardCalculator: rewardCalculator,
		nonceService:     NewNonceService(),
		runnerService:    runnerService,
	}
}

func (s *TaskService) SetRewardClient(client ports.RewardClient) {
	s.rewardClient = client
}

func (s *TaskService) CreateTask(ctx context.Context, task *models.Task) error {
	log := gologger.WithComponent("task_service")

	if err := task.Validate(); err != nil {
		log.Error().Err(err).
			Interface("task", map[string]interface{}{
				"title":  task.Title,
				"type":   task.Type,
				"config": task.Config,
			}).Msg("Invalid task")
		return ErrInvalidTask
	}

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if task.Status == "" {
		task.Status = models.TaskStatusPending
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = time.Now()

	if err := s.repo.Create(ctx, task); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to create task")
		return err
	}

	// After creating the task, find available runners and notify them
	runners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list available runners")
		// Don't return error here, as task creation was successful
	} else if len(runners) > 0 {
		// Try to assign the task to available runners
		if err := s.assignTasksToRunner([]*models.Task{task}, runners); err != nil {
			log.Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to notify runners about new task")
			// Don't return error as task creation was successful
		}
	}

	return nil
}

func (s *TaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	log := gologger.WithComponent("task_service")

	tasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list available tasks")
		return nil, err
	}

	availableTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.Status == models.TaskStatusPending {
			availableTasks = append(availableTasks, task)
		}
	}

	return availableTasks, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID string, deviceID string) error {
	log := gologger.WithComponent("task_service")

	assignKey := "assign_" + taskID + "_" + deviceID
	if _, exists := s.notificationInProgress.LoadOrStore(assignKey, true); exists {
		log.Info().
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Msg("Task assignment already in progress, skipping duplicate")
		return nil
	}
	defer s.notificationInProgress.Delete(assignKey)

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		return err
	}

	runner, err := s.runnerService.GetRunner(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("invalid runner ID: %w", err)
	}

	if runner.TaskID != nil && *runner.TaskID == task.ID {
		log.Info().
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Msg("Task already assigned to this runner, skipping reassignment")
		return nil
	}

	if task.Status == models.TaskStatusRunning {
		log.Warn().
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Msg("Task is already running, possibly assigned to another runner")
		return errors.New("task unavailable")
	}

	if task.Status == models.TaskStatusCompleted {
		log.Warn().
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Msg("Task is already completed")
		return errors.New("task unavailable")
	}

	if task.Status != models.TaskStatusPending {
		return errors.New("task unavailable")
	}

	if task.Type == models.TaskTypeDocker && (task.Environment == nil || task.Environment.Type != "docker") {
		return errors.New("invalid docker config")
	}

	err = s.assignTasksToRunner([]*models.Task{task}, []*models.Runner{runner})
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Msg("Failed to assign task to runner")
		return err
	}

	return nil
}

func (s *TaskService) GetTaskReward(ctx context.Context, taskID string) (float64, error) {
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return 0, fmt.Errorf("invalid task ID format: %w", err)
	}

	result, err := s.repo.GetTaskResult(ctx, taskUUID)
	if err != nil {
		return 0, err
	}

	if result == nil {
		return 0, fmt.Errorf("task result not found")
	}

	return result.Reward, nil
}

func (s *TaskService) GetTasks(ctx context.Context) ([]models.Task, error) {
	tasks, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func (s *TaskService) StartTask(ctx context.Context, id string) error {
	log := gologger.WithComponent("task_service")
	log.Debug().Str("task_id", id).Msg("Attempting to start task")

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Invalid task ID format")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Failed to get task")
		return err
	}

	if task.Status == models.TaskStatusRunning {
		log.Info().Str("task_id", id).Msg("Task is already running")
		return nil
	}

	if task.Status == models.TaskStatusCompleted {
		log.Warn().Str("task_id", id).Msg("Task is already completed")
		return fmt.Errorf("task already completed")
	}

	taskKey := "start_" + id
	if _, exists := s.notificationInProgress.LoadOrStore(taskKey, true); exists {
		log.Info().Str("task_id", id).Msg("Task start operation already in progress, skipping duplicate")
		return nil
	}
	defer s.notificationInProgress.Delete(taskKey)

	task.Status = models.TaskStatusRunning
	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Failed to update task status")
		return err
	}

	log.Info().Str("task_id", id).Msg("Task started successfully")
	return nil
}

func (s *TaskService) CompleteTask(ctx context.Context, id string) error {
	log := gologger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		return err
	}

	// Check if task is already completed
	if task.Status == models.TaskStatusCompleted {
		log.Info().
			Str("task_id", id).
			Msg("Task already completed, ignoring duplicate completion request")
		return nil
	}

	// Find and update the runner that was assigned to this task
	runners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusBusy)
	if err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Failed to list runners")
		return err
	}

	for _, runner := range runners {
		if runner.TaskID != nil && *runner.TaskID == taskUUID {
			runner.Status = models.RunnerStatusOnline
			runner.TaskID = nil
			runner.Task = nil
			if _, err := s.runnerService.UpdateRunner(ctx, runner); err != nil {
				log.Error().Err(err).
					Str("runner_id", runner.DeviceID).
					Str("task_id", id).
					Msg("Failed to update runner status after task completion")
				// Continue with task completion even if runner update fails
			} else {
				log.Info().
					Str("runner_id", runner.DeviceID).
					Str("task_id", id).
					Msg("Runner status updated to online after task completion")
			}
			break
		}
	}

	task.Status = models.TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now

	if err := s.repo.Update(ctx, task); err != nil {
		return err
	}

	return nil
}

func (s *TaskService) GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID format: %w", err)
	}

	result, err := s.repo.GetTaskResult(ctx, taskUUID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *TaskService) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	log := gologger.WithComponent("task_service")

	if result == nil {
		return fmt.Errorf("invalid task result: result cannot be nil")
	}

	task, err := s.repo.Get(ctx, result.TaskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Check if task is already completed to prevent duplicate submissions
	if task.Status == models.TaskStatusCompleted {
		return nil
	}

	if !s.nonceService.VerifyNonce(task.Nonce, result.Output) {
		log.Error().
			Str("task_id", result.TaskID.String()).
			Str("nonce", task.Nonce).
			Msg("Task result verification failed: nonce not found in output")

		task.Status = models.TaskStatusNotVerified
		if err := s.repo.Update(ctx, task); err != nil {
			log.Error().Err(err).
				Str("task_id", result.TaskID.String()).
				Msg("Failed to update task status to not verified")
		}
		return fmt.Errorf("invalid task result: nonce verification failed")
	}

	log.Info().
		Str("task_id", result.TaskID.String()).
		Str("nonce", task.Nonce).
		Msg("Task result verification passed")

	if result.ExitCode == 0 {
		metrics := ports.ResourceMetrics{
			CPUSeconds:      result.CPUSeconds,
			EstimatedCycles: result.EstimatedCycles,
			MemoryGBHours:   result.MemoryGBHours,
			StorageGB:       result.StorageGB,
			NetworkDataGB:   result.NetworkDataGB,
		}
		result.Reward = s.rewardCalculator.CalculateReward(metrics)
		task.Status = models.TaskStatusCompleted
		now := time.Now()
		task.CompletedAt = &now
	} else {
		task.Status = models.TaskStatusFailed
		result.Reward = 0
	}

	if err := s.repo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		return fmt.Errorf("failed to save task result: %w", err)
	}

	if result.ExitCode == 0 && s.rewardClient != nil {
		if err := s.rewardClient.DistributeRewards(result); err != nil {
			log.Error().Err(err).
				Str("task_id", result.TaskID.String()).
				Float64("reward", result.Reward).
				Msg("Failed to distribute reward")
			return fmt.Errorf("failed to distribute reward: %w", err)
		}
	}

	return nil
}

func (s *TaskService) MonitorTasks() {
	log := gologger.WithComponent("task_monitor")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	log.Debug().Msg("Monitoring task statuses")

	// Only monitor for stuck or timed out tasks
	tasks, err := s.repo.ListByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		if strings.Contains(err.Error(), "database is closed") {
			log.Info().Msg("Database closed, skipping task monitoring during shutdown")
			return
		}
		log.Error().Err(err).Msg("Failed to list tasks")
		return
	}

	for _, task := range tasks {
		if ctx.Err() != nil {
			log.Info().Msg("Context cancelled, stopping task monitoring")
			return
		}

		// Add monitoring logic here if needed
		// For example, check for stuck tasks, timeouts, etc.
		log.Debug().
			Str("task_id", task.ID.String()).
			Str("status", string(task.Status)).
			Msg("Monitoring task status")
	}
}

func (s *TaskService) assignTasksToRunner(tasks []*models.Task, runners []*models.Runner) error {
	log := gologger.WithComponent("task_assign")

	batchKey := fmt.Sprintf("batch_assign_%d", time.Now().UnixNano())
	if _, exists := s.notificationInProgress.LoadOrStore(batchKey, true); exists {
		log.Info().Msg("Batch assignment already in progress, skipping duplicate")
		return nil
	}
	defer s.notificationInProgress.Delete(batchKey)

	availableRunnerList := make([]*models.Runner, 0, len(runners))
	for _, runner := range runners {
		if runner.Status != models.RunnerStatusBusy {
			availableRunnerList = append(availableRunnerList, runner)
		}
	}

	availableRunners := len(availableRunnerList)
	if availableRunners == 0 {
		log.Warn().Msg("No available runners to assign tasks")
		return nil
	}

	pendingTasks := make([]*models.Task, 0, len(tasks))
	ctx := context.Background()

	for _, task := range tasks {
		currentTask, err := s.repo.Get(ctx, task.ID)
		if err != nil {
			log.Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to get current task status")
			continue
		}

		if currentTask.Status == models.TaskStatusPending {
			pendingTasks = append(pendingTasks, currentTask)
		} else {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("status", string(currentTask.Status)).
				Msg("Task no longer in pending state, skipping assignment")
		}
	}

	if len(pendingTasks) == 0 {
		log.Info().Msg("No pending tasks to assign")
		return nil
	}

	log.Info().
		Int("available_runners", availableRunners).
		Int("tasks", len(pendingTasks)).
		Msg("Starting task assignment")

	for _, task := range pendingTasks {
		for _, runner := range availableRunnerList {
			if runner.Status == models.RunnerStatusBusy {
				continue
			}

			task.Status = models.TaskStatusRunning
			task.UpdatedAt = time.Now()
			if err := s.repo.Update(ctx, task); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to update task status to running")
				continue
			}

			runner.Status = models.RunnerStatusBusy
			runner.TaskID = &task.ID
			runner.Task = task

			updatedRunner, err := s.runnerService.UpdateRunner(ctx, runner)
			if err != nil {
				log.Error().Err(err).
					Str("runner_id", runner.DeviceID).
					Str("task_id", task.ID.String()).
					Msg("Failed to update runner status")

				task.Status = models.TaskStatusPending
				if revertErr := s.repo.Update(ctx, task); revertErr != nil {
					log.Error().Err(revertErr).
						Str("task_id", task.ID.String()).
						Msg("Failed to revert task status")
				}
				continue
			}

			if err := s.notifyRunnerAboutTask(updatedRunner, task); err != nil {
				log.Error().
					Err(err).
					Str("runner_id", updatedRunner.DeviceID).
					Str("task_id", task.ID.String()).
					Msg("Failed to notify runner about task")
			} else {
				log.Info().
					Str("runner_id", updatedRunner.DeviceID).
					Str("task_id", task.ID.String()).
					Str("webhook", updatedRunner.Webhook).
					Msg("Runner notified about task")
			}

			break
		}
	}

	return nil
}

func (s *TaskService) notifyRunnerAboutTask(runner *models.Runner, task *models.Task) error {
	log := gologger.WithComponent("task_assign")

	if runner.Webhook == "" {
		log.Warn().
			Str("runner_id", runner.DeviceID).
			Msg("Runner has no webhook URL registered")
		return nil
	}

	taskKey := "notify_" + task.ID.String() + "_" + runner.DeviceID
	if _, exists := s.notificationInProgress.LoadOrStore(taskKey, true); exists {
		log.Info().
			Str("task_id", task.ID.String()).
			Str("runner_id", runner.DeviceID).
			Msg("Notification already in progress for this task, skipping duplicate")
		return nil
	}

	log.Info().
		Str("runner_id", runner.DeviceID).
		Str("task_id", task.ID.String()).
		Str("webhook_url", runner.Webhook).
		Msg("Notifying runner about task")

	go func() {
		defer s.notificationInProgress.Delete(taskKey)

		ctx := context.Background()
		backoff := time.Second
		maxRetries := 3
		var notificationSent bool

		currentTask, err := s.repo.Get(ctx, task.ID)
		if err != nil {
			log.Error().Err(err).
				Str("task_id", task.ID.String()).
				Msg("Failed to get task status before webhook notification")
			return
		}

		if currentTask.Status != models.TaskStatusRunning {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("status", string(currentTask.Status)).
				Msg("Task no longer in running state, skipping webhook notification")
			return
		}

		currentRunner, err := s.runnerService.GetRunner(ctx, runner.DeviceID)
		if err != nil {
			log.Error().Err(err).
				Str("runner_id", runner.DeviceID).
				Msg("Failed to get runner status before notification")
			return
		}

		if currentRunner.TaskID == nil || *currentRunner.TaskID != task.ID {
			log.Info().
				Str("runner_id", runner.DeviceID).
				Str("task_id", task.ID.String()).
				Msg("Runner no longer assigned to this task, skipping notification")
			return
		}

		for i := 0; i < maxRetries; i++ {
			if i > 0 {
				currentTask, err := s.repo.Get(ctx, task.ID)
				if err != nil {
					log.Error().Err(err).
						Str("task_id", task.ID.String()).
						Msg("Failed to get task status during retry")
					return
				}

				if currentTask.Status != models.TaskStatusRunning {
					log.Info().
						Str("task_id", task.ID.String()).
						Str("status", string(currentTask.Status)).
						Msg("Task no longer in running state, stopping retries")
					return
				}

				currentRunner, err := s.runnerService.GetRunner(ctx, runner.DeviceID)
				if err != nil {
					log.Error().Err(err).
						Str("runner_id", runner.DeviceID).
						Msg("Failed to get runner status during retry")
					return
				}

				if currentRunner.TaskID == nil || *currentRunner.TaskID != task.ID {
					log.Info().
						Str("runner_id", runner.DeviceID).
						Str("task_id", task.ID.String()).
						Msg("Runner no longer assigned to this task, stopping retries")
					return
				}
			}

			err = s.sendWebhookNotification(ctx, runner, task)
			if err == nil {
				notificationSent = true
				log.Info().
					Str("runner_id", runner.DeviceID).
					Str("task_id", task.ID.String()).
					Msg("Webhook notification sent successfully")
				break
			}

			if i < maxRetries-1 {
				log.Warn().
					Err(err).
					Int("retry", i+1).
					Str("runner_id", runner.DeviceID).
					Str("task_id", task.ID.String()).
					Str("webhook_url", runner.Webhook).
					Msg("Webhook notification failed, retrying")
				time.Sleep(backoff)
				backoff *= 2 // Exponential backoff
				continue
			}

			log.Error().
				Err(err).
				Str("runner_id", runner.DeviceID).
				Str("task_id", task.ID.String()).
				Str("webhook_url", runner.Webhook).
				Msg("Failed to notify runner about task after all retries")
		}

		if !notificationSent {
			currentTask, err := s.repo.Get(ctx, task.ID)
			if err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to get task status after webhook failure")
				return
			}

			if currentTask.Status == models.TaskStatusRunning {
				task.Status = models.TaskStatusFailed
				if err := s.repo.Update(ctx, task); err != nil {
					log.Error().Err(err).
						Str("task_id", task.ID.String()).
						Msg("Failed to update task status after webhook failure")
				}
			}
		}
	}()

	// Return immediately since notification is async
	return nil
}

func (s *TaskService) sendWebhookNotification(ctx context.Context, runner *models.Runner, task *models.Task) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
			DisableKeepAlives:  false,
			MaxConnsPerHost:    10,
			ForceAttemptHTTP2:  true,
		},
	}

	// Create payload matching runner's expected format
	payload := map[string]interface{}{
		"type":    "available_tasks",
		"payload": []*models.Task{task}, // Send as array of tasks
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", runner.Webhook, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Parity-Server/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
