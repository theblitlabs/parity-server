package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
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

type TaskService struct {
	repo                   TaskRepository
	rewardCalculator       ports.RewardCalculator
	rewardClient           ports.RewardClient
	nonceService           *NonceService
	runnerService          *RunnerService
	notificationInProgress sync.Map // Used to track in-progress notifications
	stopChan               chan struct{}
	wg                     sync.WaitGroup
}

func NewTaskService(repo TaskRepository, rewardCalculator ports.RewardCalculator, runnerService *RunnerService) *TaskService {
	return &TaskService{
		repo:             repo,
		rewardCalculator: rewardCalculator,
		nonceService:     NewNonceService(),
		runnerService:    runnerService,
		stopChan:         make(chan struct{}),
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

	runners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list available runners")
	} else if len(runners) > 0 {
		// Filter out runners that are already assigned tasks
		availableRunners := make([]*models.Runner, 0)
		for _, runner := range runners {
			if runner.TaskID == nil {
				availableRunners = append(availableRunners, runner)
			}
		}

		if len(availableRunners) > 0 {
			sortRunnersByLoad(availableRunners)

			// Try to assign the task to the first compatible runner
			for _, runner := range availableRunners {
				if isRunnerCompatibleWithTask(runner) {
					if err := s.assignTaskToRunner(ctx, task, runner); err != nil {
						log.Error().Err(err).
							Str("task_id", task.ID.String()).
							Str("runner_id", runner.DeviceID).
							Msg("Failed to assign task to runner")
					}
					break
				}
			}
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

	return s.assignTaskToRunner(ctx, task, runner)
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

	if task.Status != models.TaskStatusPending {
		log.Error().
			Str("task_id", id).
			Str("status", string(task.Status)).
			Msg("Task is not in pending status")
		return fmt.Errorf("task is not in pending status")
	}

	task.Status = models.TaskStatusRunning
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Failed to update task status")
		return err
	}

	log.Info().
		Str("task_id", id).
		Str("status", string(task.Status)).
		Msg("Task started successfully")

	return nil
}

func (s *TaskService) CompleteTask(ctx context.Context, id string) error {
	log := gologger.WithComponent("task_service")
	log.Debug().Str("task_id", id).Msg("Attempting to complete task")

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

	if task.Status != models.TaskStatusRunning {
		log.Error().
			Str("task_id", id).
			Str("status", string(task.Status)).
			Msg("Task is not in running status")
		return fmt.Errorf("task is not in running status")
	}

	task.Status = models.TaskStatusCompleted
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Failed to update task status")
		return err
	}

	log.Info().
		Str("task_id", id).
		Str("status", string(task.Status)).
		Msg("Task completed successfully")

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

	if result.TaskID == uuid.Nil {
		return fmt.Errorf("invalid task ID")
	}

	task, err := s.repo.Get(ctx, result.TaskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskStatusRunning &&
		task.Status != models.TaskStatusCompleted &&
		task.Status != models.TaskStatusPending {
		log.Warn().
			Str("task_id", result.TaskID.String()).
			Str("current_status", string(task.Status)).
			Msg("Attempting to save result for task that is not in running, completed, or pending status")
		return fmt.Errorf("task is not in a valid status: %s", task.Status)
	}

	// Get the runner ID from multiple possible sources to ensure we have it
	runnerID := task.RunnerID
	if runnerID == "" {
		runnerID = result.DeviceID // Try device ID from result
	}
	if runnerID == "" {
		runnerID = result.SolverDeviceID // Try solver device ID from result
	}

	// Store the RunnerID in the task result's device fields if not already set
	if result.DeviceID == "" {
		result.DeviceID = runnerID
	}
	if result.SolverDeviceID == "" {
		result.SolverDeviceID = runnerID
	}

	// First, clear the runner's TaskID - this must happen before any other operations
	if runnerID != "" {
		log.Info().
			Str("task_id", result.TaskID.String()).
			Str("runner_id", runnerID).
			Msg("Attempting to clear runner TaskID")

		runner, err := s.runnerService.GetRunner(ctx, runnerID)
		if err != nil {
			log.Error().Err(err).
				Str("task_id", result.TaskID.String()).
				Str("runner_id", runnerID).
				Msg("Failed to get runner info for clearing TaskID")
		} else {
			log.Info().
				Str("task_id", result.TaskID.String()).
				Str("runner_id", runnerID).
				Interface("current_task_id", runner.TaskID).
				Msg("Found runner, clearing TaskID")

			// Clear the TaskID and update the runner immediately
			runner.TaskID = nil
			runner.Status = models.RunnerStatusOnline // Ensure runner stays online
			updatedRunner, err := s.runnerService.UpdateRunner(ctx, runner)
			if err != nil {
				log.Error().Err(err).
					Str("task_id", result.TaskID.String()).
					Str("runner_id", runnerID).
					Msg("Failed to clear runner TaskID")
			} else {
				if updatedRunner.TaskID != nil {
					log.Error().
						Str("task_id", result.TaskID.String()).
						Str("runner_id", runnerID).
						Interface("task_id_after_update", updatedRunner.TaskID).
						Msg("Runner TaskID not cleared properly")
				} else {
					log.Info().
						Str("task_id", result.TaskID.String()).
						Str("runner_id", runnerID).
						Msg("Successfully cleared runner TaskID after task completion")
				}
			}
		}
	} else {
		log.Warn().
			Str("task_id", result.TaskID.String()).
			Msg("No runner ID found in task or result")
		return fmt.Errorf("no runner ID found to clear TaskID")
	}

	// Calculate reward
	metrics := ports.ResourceMetrics{
		CPUSeconds:      result.CPUSeconds,
		EstimatedCycles: result.EstimatedCycles,
		MemoryGBHours:   result.MemoryGBHours,
		StorageGB:       result.StorageGB,
		NetworkDataGB:   result.NetworkDataGB,
	}

	reward := s.rewardCalculator.CalculateReward(metrics)
	result.Reward = reward

	// Save the task result and update task status
	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		log.Error().Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to save task result")
		return err
	}

	if task.Status != models.TaskStatusCompleted {
		task.Status = models.TaskStatusCompleted
		task.UpdatedAt = time.Now()
		task.RunnerID = runnerID // Preserve the RunnerID

		if task.CompletedAt == nil {
			now := time.Now()
			task.CompletedAt = &now
		}

		if err := s.repo.Update(ctx, task); err != nil {
			log.Error().Err(err).
				Str("task_id", result.TaskID.String()).
				Msg("Failed to update task status")
		} else {
			log.Info().
				Str("task_id", result.TaskID.String()).
				Str("runner_id", runnerID).
				Msg("Task marked as completed after receiving results")
		}
	}

	// Double-check that the runner's TaskID is cleared
	runner, err := s.runnerService.GetRunner(ctx, runnerID)
	if err == nil && runner.TaskID != nil {
		log.Warn().
			Str("task_id", result.TaskID.String()).
			Str("runner_id", runnerID).
			Interface("task_id_after_completion", runner.TaskID).
			Msg("Runner still has TaskID after completion, attempting to clear again")

		runner.TaskID = nil
		runner.Status = models.RunnerStatusOnline
		if _, err := s.runnerService.UpdateRunner(ctx, runner); err != nil {
			log.Error().Err(err).
				Str("task_id", result.TaskID.String()).
				Str("runner_id", runnerID).
				Msg("Failed to clear runner TaskID in final check")
		}
	}

	// Process rewards in a goroutine to not block task assignment
	if s.rewardClient != nil {
		go func() {
			if err := s.rewardClient.DistributeRewards(result); err != nil {
				log.Error().Err(err).
					Str("task_id", result.TaskID.String()).
					Float64("reward", result.Reward).
					Msg("Failed to distribute rewards")
			}
		}()
	}

	// Check for pending tasks to assign to this runner immediately
	go func() {
		if err := s.checkAndAssignPendingTasksToRunner(context.Background(), runnerID); err != nil {
			log.Error().Err(err).
				Str("runner_id", runnerID).
				Msg("Failed to check and assign pending tasks to runner")
		}
	}()

	return nil
}

func (s *TaskService) StartMonitoring() {
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.MonitorTasks()
	}()
	go func() {
		defer s.wg.Done()
		s.MonitorPendingTasks()
	}()
}

func (s *TaskService) StopMonitoring() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *TaskService) MonitorPendingTasks() {
	log := gologger.WithComponent("task_service")
	log.Info().Msg("Starting pending tasks monitoring (backup mechanism)")

	// this is a backup mechanism to ensure that pending tasks are assigned to runners
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
	if err := s.processPendingTasks(); err != nil {
		log.Error().Err(err).Msg("Error processing pending tasks on startup")
	}

	for {
		select {
		case <-s.stopChan:
			log.Info().Msg("Stopping pending tasks monitoring")
			return
		case <-ticker.C:
			log.Debug().Msg("Running backup pending tasks check")
			if err := s.processPendingTasks(); err != nil {
				log.Error().Err(err).Msg("Error processing pending tasks")
			}
		}
	}
}

func (s *TaskService) processPendingTasks() error {
	log := gologger.WithComponent("task_service")

	// Get all pending tasks
	pendingTasks, err := s.repo.ListByStatus(context.Background(), models.TaskStatusPending)
	if err != nil {
		return fmt.Errorf("failed to list pending tasks: %w", err)
	}

	log.Info().
		Int("pending_tasks_count", len(pendingTasks)).
		Msg("Found pending tasks")

	if len(pendingTasks) == 0 {
		return nil
	}

	// Get all available runners
	runners, err := s.runnerService.ListRunnersByStatus(context.Background(), models.RunnerStatusOnline)
	if err != nil {
		return fmt.Errorf("failed to list available runners: %w", err)
	}

	log.Info().
		Int("online_runners_count", len(runners)).
		Msg("Found online runners")

	if len(runners) == 0 {
		return nil
	}

	// Filter out runners that are not online
	availableRunners := make([]*models.Runner, 0)
	for _, runner := range runners {
		if runner.Status == models.RunnerStatusOnline {
			availableRunners = append(availableRunners, runner)
		} else {
			log.Debug().
				Str("runner_id", runner.DeviceID).
				Str("status", string(runner.Status)).
				Msg("Runner is not online")
		}
	}

	log.Info().
		Int("available_runners_count", len(availableRunners)).
		Msg("Found available runners after filtering")

	if len(availableRunners) == 0 {
		return nil
	}

	sortRunnersByLoad(availableRunners)

	// Try to assign each pending task to available runners
	for _, task := range pendingTasks {
		log.Info().
			Str("task_id", task.ID.String()).
			Str("task_status", string(task.Status)).
			Str("task_type", string(task.Type)).
			Msg("Attempting to assign task")

		assigned := false
		for _, runner := range availableRunners {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("runner_id", runner.DeviceID).
				Str("runner_status", string(runner.Status)).
				Msg("Checking runner compatibility")

			if isRunnerCompatibleWithTask(runner) {
				if err := s.assignTaskToRunner(context.Background(), task, runner); err != nil {
					log.Error().Err(err).
						Str("task_id", task.ID.String()).
						Str("runner_id", runner.DeviceID).
						Msg("Failed to assign task to runner")
					continue
				}
				// Remove the runner from available runners after assignment
				for i, r := range availableRunners {
					if r.DeviceID == runner.DeviceID {
						availableRunners = append(availableRunners[:i], availableRunners[i+1:]...)
						break
					}
				}
				assigned = true
				break
			} else {
				log.Info().
					Str("task_id", task.ID.String()).
					Str("runner_id", runner.DeviceID).
					Msg("Runner not compatible with task")
			}
		}

		if !assigned {
			log.Info().
				Str("task_id", task.ID.String()).
				Msg("No compatible runners found for task")
		}
	}

	return nil
}

func (s *TaskService) assignTaskToRunner(ctx context.Context, task *models.Task, runner *models.Runner) error {
	log := gologger.WithComponent("task_service")

	task.RunnerID = runner.DeviceID
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task with runner ID: %w", err)
	}

	runner.TaskID = &task.ID
	if _, err := s.runnerService.UpdateRunner(ctx, runner); err != nil {
		task.RunnerID = ""
		if revertErr := s.repo.Update(ctx, task); revertErr != nil {
			log.Error().Err(revertErr).
				Str("task_id", task.ID.String()).
				Msg("Failed to revert task runner ID")
		}
		return fmt.Errorf("failed to update runner with task ID: %w", err)
	}

	if err := s.notifyRunnerAboutTask(runner, task); err != nil {
		log.Warn().Err(err).
			Str("task_id", task.ID.String()).
			Str("runner_id", runner.DeviceID).
			Msg("Failed to notify runner about task, but keeping assignment")
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("runner_id", runner.DeviceID).
		Msg("Successfully assigned task to runner")

	return nil
}

func (s *TaskService) notifyRunnerAboutTask(runner *models.Runner, task *models.Task) error {
	log := gologger.WithComponent("task_service")

	if runner.Webhook == "" {
		log.Debug().
			Str("runner_id", runner.DeviceID).
			Msg("Runner has no webhook URL configured")
		return nil
	}

	nonce := s.nonceService.GenerateNonce()
	task.Nonce = nonce

	if err := s.repo.Update(context.Background(), task); err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Msg("Failed to update task with nonce")
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.sendWebhookNotification(ctx, runner, task); err != nil {
		log.Warn().Err(err).
			Str("task_id", task.ID.String()).
			Str("runner_id", runner.DeviceID).
			Msg("Failed to send webhook notification but continuing")
	}

	return nil
}

func (s *TaskService) sendWebhookNotification(ctx context.Context, runner *models.Runner, task *models.Task) error {
	log := gologger.WithComponent("task_service")

	var taskConfig map[string]interface{}
	if task.Config != nil {
		if err := json.Unmarshal(task.Config, &taskConfig); err != nil {
			log.Error().Err(err).
				Str("task_id", task.ID.String()).
				Msg("Failed to unmarshal task config")
			return fmt.Errorf("failed to unmarshal task config: %w", err)
		}
	}

	var environmentMap map[string]interface{}
	if task.Environment != nil {
		environmentMap = map[string]interface{}{
			"type":   task.Environment.Type,
			"config": task.Environment.Config,
		}
	}

	taskPayload := map[string]interface{}{
		"id":          task.ID.String(),
		"title":       task.Title,
		"description": task.Description,
		"type":        task.Type,
		"config":      taskConfig,
		"environment": environmentMap,
		"nonce":       task.Nonce,
		"status":      task.Status,
	}

	if task.CompletedAt != nil {
		taskPayload["completed_at"] = task.CompletedAt.Format(time.RFC3339)
	}

	payload := map[string]interface{}{
		"type":    "available_tasks",
		"payload": taskPayload,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("runner_id", runner.DeviceID).
		Msg("Sending webhook notification")

	req, err := http.NewRequestWithContext(ctx, "POST", runner.Webhook, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Runner-ID", runner.DeviceID)

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook notification failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Debug().
		Str("task_id", task.ID.String()).
		Str("runner_id", runner.DeviceID).
		Msg("Webhook notification sent successfully")

	return nil
}

func sortRunnersByLoad(runners []*models.Runner) {
	sort.Slice(runners, func(i, j int) bool {
		return true // Simple round-robin for now
	})
}

func isRunnerCompatibleWithTask(runner *models.Runner) bool {
	log := gologger.WithComponent("task_service")

	if runner.Status != models.RunnerStatusOnline {
		log.Info().
			Str("runner_id", runner.DeviceID).
			Str("status", string(runner.Status)).
			Msg("Runner not compatible: not online")
		return false
	}

	if runner.TaskID != nil {
		log.Info().
			Str("runner_id", runner.DeviceID).
			Interface("task_id", runner.TaskID).
			Msg("Runner not compatible: has task assigned")
		return false
	}

	log.Info().
		Str("runner_id", runner.DeviceID).
		Msg("Runner is compatible with task")
	return true
}

func (s *TaskService) MonitorTasks() {
	log := gologger.WithComponent("task_service")
	log.Info().Msg("Starting task monitoring")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			log.Info().Msg("Stopping task monitoring")
			return
		case <-ticker.C:
			if err := s.checkStalledTasks(); err != nil {
				log.Error().Err(err).Msg("Error checking stalled tasks")
			}
		}
	}
}

func (s *TaskService) checkStalledTasks() error {
	log := gologger.WithComponent("task_service")

	tasks, err := s.repo.ListByStatus(context.Background(), models.TaskStatusRunning)
	if err != nil {
		return fmt.Errorf("failed to list running tasks: %w", err)
	}

	for _, task := range tasks {
		if task.UpdatedAt.Add(5 * time.Minute).Before(time.Now()) {
			log.Warn().
				Str("task_id", task.ID.String()).
				Time("last_update", task.UpdatedAt).
				Msg("Task appears to be stalled")

			if err := s.handleStalledTask(task); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to handle stalled task")
			}
		}
	}

	return nil
}

func (s *TaskService) handleStalledTask(task *models.Task) error {
	log := gologger.WithComponent("task_service")

	runner, err := s.runnerService.GetRunner(context.Background(), task.RunnerID)
	if err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Str("runner_id", task.RunnerID).
			Msg("Failed to get runner info")
		return err
	}

	runner.Status = models.RunnerStatusOffline
	runner.TaskID = nil
	if _, err := s.runnerService.UpdateRunner(context.Background(), runner); err != nil {
		log.Error().Err(err).
			Str("runner_id", runner.DeviceID).
			Msg("Failed to update runner status")
		return err
	}

	task.Status = models.TaskStatusPending
	task.RunnerID = ""
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(context.Background(), task); err != nil {
		return fmt.Errorf("failed to reset task status: %w", err)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("runner_id", runner.DeviceID).
		Msg("Reset stalled task to pending status")

	return nil
}

func (s *TaskService) checkAndAssignPendingTasksToRunner(ctx context.Context, runnerID string) error {
	log := gologger.WithComponent("task_service")

	runner, err := s.runnerService.GetRunner(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to get runner: %w", err)
	}

	if runner.Status != models.RunnerStatusOnline || runner.TaskID != nil {
		return nil
	}

	pendingTasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		return fmt.Errorf("failed to list pending tasks: %w", err)
	}

	if len(pendingTasks) == 0 {
		return nil
	}

	log.Info().
		Str("runner_id", runnerID).
		Int("pending_tasks", len(pendingTasks)).
		Msg("Checking pending tasks for runner")

	for _, task := range pendingTasks {
		if isRunnerCompatibleWithTask(runner) {
			if err := s.assignTaskToRunner(ctx, task, runner); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID.String()).
					Str("runner_id", runner.DeviceID).
					Msg("Failed to assign task to runner")
				continue
			}
			log.Info().
				Str("task_id", task.ID.String()).
				Str("runner_id", runner.DeviceID).
				Msg("Successfully assigned pending task to runner")
			return nil
		}
	}

	return nil
}
