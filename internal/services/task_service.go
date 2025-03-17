package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
	"github.com/theblitlabs/parity-server/internal/models"
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

type RewardCalculatorService interface {
	CalculateReward(resourceMetrics ResourceMetrics) float64
}

type TaskService struct {
	repo                   TaskRepository
	rewardCalculator       *RewardCalculator
	rewardClient           RewardClient
	nonceService           *NonceService
	runnerService          *RunnerService
	notificationInProgress sync.Map // Used to track in-progress notifications
}

func NewTaskService(repo TaskRepository, rewardCalculator *RewardCalculator, runnerService *RunnerService) *TaskService {
	return &TaskService{
		repo:             repo,
		rewardCalculator: rewardCalculator,
		nonceService:     NewNonceService(),
		runnerService:    runnerService,
	}
}

func (s *TaskService) SetRewardClient(client RewardClient) {
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

	log.Info().Str("task_id", task.ID.String()).Msg("Attempting to assign newly created task to runners")

	runners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list available runners for immediate task assignment")
		return nil
	}

	if len(runners) == 0 {
		log.Warn().Msg("No available runners to assign newly created task")
		return nil
	}

	tasks := []*models.Task{task}

	if err := s.assignTasksToRunner(tasks, runners); err != nil {
		log.Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to assign newly created task")
	} else {
		log.Info().Str("task_id", task.ID.String()).Msg("Successfully assigned newly created task")
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

	if task.Status != models.TaskStatusPending {
		return errors.New("task unavailable")
	}

	if task.Type == models.TaskTypeDocker && (task.Environment == nil || task.Environment.Type != "docker") {
		return errors.New("invalid docker config")
	}

	task.Status = models.TaskStatusRunning
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task", taskID).Msg("Failed to assign task")
		return err
	}

	runner.Status = models.RunnerStatusBusy
	runner.TaskID = &task.ID
	runner.Task = task

	if _, err := s.runnerService.UpdateRunner(ctx, runner); err != nil {
		log.Error().Err(err).Str("runner", deviceID).Msg("Failed to update runner")
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

	// Check if task is already running or completed
	if task.Status == models.TaskStatusRunning {
		log.Warn().Str("task_id", id).Msg("Task is already running")
		return nil // Return success to avoid errors with already-running tasks
	}

	if task.Status == models.TaskStatusCompleted {
		log.Warn().Str("task_id", id).Msg("Task is already completed")
		return fmt.Errorf("task already completed")
	}

	// Only pending tasks can be started
	if task.Status != models.TaskStatusPending {
		log.Warn().
			Str("task_id", id).
			Str("current_status", string(task.Status)).
			Msg("Cannot start task with current status")
		return fmt.Errorf("cannot start task with status %s", task.Status)
	}

	// Update task status to running
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
		log.Info().
			Str("task_id", result.TaskID.String()).
			Msg("Task already completed, ignoring duplicate result submission")
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

	// Calculate reward based on resource metrics
	if result.ExitCode == 0 {
		metrics := ResourceMetrics{
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

	log.Debug().Msg("Checking for pending tasks")

	tasks, err := s.repo.ListByStatus(context.Background(), models.TaskStatusPending)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks")
		return
	}

	runners, err := s.runnerService.ListRunnersByStatus(context.Background(), models.RunnerStatusOnline)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list runners")
		return
	}

	log.Debug().
		Int("pending_tasks", len(tasks)).
		Int("online_runners", len(runners)).
		Msg("Task assignment status")

	if len(runners) == 0 {
		log.Debug().Msg("No runners available")
		return
	}

	if len(tasks) == 0 {
		log.Debug().Msg("No pending tasks available")
		return
	}

	err = s.assignTasksToRunner(tasks, runners)
	if err != nil {
		log.Error().Err(err).Msg("Failed to assign tasks to runners")
		return
	}
}

// CheckAndAssignTasks is called when a runner becomes available
func (s *TaskService) CheckAndAssignTasks() {
	log := gologger.WithComponent("task_monitor")
	log.Info().Msg("Runner available - checking for pending tasks")
	s.MonitorTasks()
}

func (s *TaskService) assignTasksToRunner(tasks []*models.Task, runners []*models.Runner) error {
	log := gologger.WithComponent("task_assign")

	var batchSize = 1
	// Filter out busy runners first
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

	var runner_iterator = 0
	var assignedTasks = 0

	log.Info().Int("available_runners", availableRunners).Int("tasks", len(tasks)).Msg("Starting task assignment")

	for i := 0; i < len(tasks) && runner_iterator < len(availableRunnerList); i++ {
		if availableRunners-batchSize < 0 {
			log.Warn().
				Int("available_runners", availableRunners).
				Int("batch_size", batchSize).
				Int("assigned_tasks", assignedTasks).
				Msg("Not enough free runners for remaining tasks")
			if assignedTasks > 0 {
				// At least some tasks were assigned, so return success
				return nil
			}
			return fmt.Errorf("not enough free runners for task")
		}

		availableRunners -= batchSize
		var assignedRunners = 0

		for runner_iterator < len(availableRunnerList) && assignedRunners < batchSize {
			runner := availableRunnerList[runner_iterator]

			// Double-check runner status before assignment
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			currentRunner, err := s.runnerService.GetRunner(ctx, runner.DeviceID)
			cancel()

			if err != nil {
				log.Error().Err(err).Str("runner_id", runner.DeviceID).Msg("Failed to get current runner status")
				runner_iterator++
				continue
			}

			// Skip if runner is no longer online
			if currentRunner.Status != models.RunnerStatusOnline {
				log.Info().
					Str("runner_id", runner.DeviceID).
					Str("status", string(currentRunner.Status)).
					Msg("Skipping runner - not in online status")
				runner_iterator++
				continue
			}

			err = s.AssignTaskToRunner(context.Background(), tasks[i].ID.String(), runner.DeviceID)
			if err != nil {
				if err.Error() == "task unavailable" {
					log.Info().
						Str("task_id", tasks[i].ID.String()).
						Msg("Task already assigned or completed, skipping")
					break // Skip to next task
				}

				log.Error().Err(err).
					Str("runner_id", runner.DeviceID).
					Str("task_id", tasks[i].ID.String()).
					Msg("Failed to assign task to runner")
				runner_iterator++
				continue
			}

			// Notify runner about the assigned task via webhook
			if runner.Webhook != "" {
				if err := s.notifyRunnerAboutTask(runner, tasks[i]); err != nil {
					log.Error().
						Err(err).
						Str("runner_id", runner.DeviceID).
						Str("task_id", tasks[i].ID.String()).
						Msg("Failed to notify runner about task")
					// Continue despite notification error - the runner will poll for tasks
				} else {
					log.Info().
						Str("runner_id", runner.DeviceID).
						Str("task_id", tasks[i].ID.String()).
						Str("webhook", runner.Webhook).
						Msg("Runner notified about task")
				}
			} else {
				log.Warn().
					Str("runner_id", runner.DeviceID).
					Msg("Runner has no webhook URL registered")
			}

			runner_iterator++
			log.Info().
				Str("runner_id", runner.DeviceID).
				Str("task_id", tasks[i].ID.String()).
				Msg("Task assigned to runner")
			assignedRunners++
			assignedTasks++
		}
	}

	if assignedTasks == 0 {
		return fmt.Errorf("could not assign any tasks")
	}

	log.Info().Int("assigned_tasks", assignedTasks).Msg("Task assignment complete")
	return nil
}

// notifyRunnerAboutTask sends a webhook notification to a runner about an assigned task
func (s *TaskService) notifyRunnerAboutTask(runner *models.Runner, task *models.Task) error {
	log := gologger.WithComponent("task_assign")
	log.Info().
		Str("runner_id", runner.DeviceID).
		Str("task_id", task.ID.String()).
		Str("webhook_url", runner.Webhook).
		Msg("Notifying runner about task")

	// Create a new background context for async webhook notifications
	// This ensures the webhook retries aren't affected by the parent context
	go func() {
		ctx := context.Background()
		backoff := time.Second
		maxRetries := 3 // Reduced from 5 to 3 retries
		var notificationSent bool

		// Check if task is still assigned to this runner and not completed
		currentTask, err := s.repo.Get(ctx, task.ID)
		if err != nil {
			log.Error().Err(err).
				Str("task_id", task.ID.String()).
				Msg("Failed to get task status before webhook notification")
			return
		}

		// Skip notification if task is not in running state
		if currentTask.Status != models.TaskStatusRunning {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("status", string(currentTask.Status)).
				Msg("Task no longer in running state, skipping webhook notification")
			return
		}

		// Check if runner is still assigned to this task
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

		// Check if this task has a notification in progress
		// This helps prevent duplicate notifications when multiple assignment attempts occur
		taskKey := task.ID.String()
		if _, exists := s.notificationInProgress.LoadOrStore(taskKey, true); exists {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("runner_id", runner.DeviceID).
				Msg("Notification already in progress for this task, skipping duplicate")
			return
		}
		defer s.notificationInProgress.Delete(taskKey)

		for i := 0; i < maxRetries; i++ {
			// Before each retry, verify task is still valid for notification
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

			// Check if runner is still assigned to this task
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

			err = s.sendWebhookNotification(ctx, runner, task)
			if err == nil {
				notificationSent = true
				log.Info().
					Str("runner_id", runner.DeviceID).
					Str("task_id", task.ID.String()).
					Msg("Webhook notification sent successfully")
				return
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

		// If notification failed after all retries but task was completed successfully,
		// don't mark it as failed since the runner might have received the task through polling
		if !notificationSent && currentTask.Status == models.TaskStatusRunning {
			task.Status = models.TaskStatusFailed
			if err := s.repo.Update(ctx, task); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to update task status after webhook failure")
			}
		}
	}()

	// Return immediately since notification is async
	return nil
}

func (s *TaskService) sendWebhookNotification(ctx context.Context, runner *models.Runner, task *models.Task) error {
	client := &http.Client{
		Timeout: 10 * time.Second, // Increased timeout for better reliability
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
