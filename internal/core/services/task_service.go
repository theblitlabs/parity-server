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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
	"github.com/theblitlabs/parity-server/internal/utils"
)

var (
	ErrInvalidTask       = errors.New("invalid task data")
	ErrTaskNotFound      = repositories.ErrTaskNotFound
	ErrTaskUnavailable   = errors.New("task unavailable")
	ErrRunnerUnavailable = errors.New("runner unavailable")
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
	GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error)
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

const (
	MinRunnersForTask        = 1
	MaxRunnersForTask        = 5
	pendingAssignmentTimeout = 45 * time.Second
)

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
	if err := task.Validate(); err != nil {
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
		return err
	}

	if s.runnerService != nil {
		s.runnerService.TriggerTaskMonitor()
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

	if runner.TaskID != nil && *runner.TaskID != task.ID {
		log.Warn().
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Str("assigned_task_id", runner.TaskID.String()).
			Msg("Runner is already processing another task")
		return ErrRunnerUnavailable
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
		return ErrTaskUnavailable
	}

	if task.Status == models.TaskStatusCompleted {
		log.Warn().
			Str("task_id", taskID).
			Str("runner_id", deviceID).
			Msg("Task is already completed")
		return ErrTaskUnavailable
	}

	if task.Status != models.TaskStatusPending {
		return ErrTaskUnavailable
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
	now := time.Now()
	task.CompletedAt = &now

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

func (s *TaskService) FailTask(ctx context.Context, id string, reason string) error {
	log := gologger.WithComponent("task_service")
	log.Debug().Str("task_id", id).Str("reason", reason).Msg("Attempting to fail task")

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

	// Allow failing tasks in any status except completed
	if task.Status == models.TaskStatusCompleted {
		log.Warn().
			Str("task_id", id).
			Str("status", string(task.Status)).
			Msg("Cannot fail task that is already completed")
		return fmt.Errorf("cannot fail task that is already completed")
	}

	task.Status = models.TaskStatusFailed
	task.UpdatedAt = time.Now()
	now := time.Now()
	task.CompletedAt = &now

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task_id", id).Msg("Failed to update task status to failed")
		return err
	}

	// Clear runner assignment if task has a runner
	if task.RunnerID != "" {
		runner, err := s.runnerService.GetRunner(ctx, task.RunnerID)
		if err == nil {
			runner.TaskID = nil
			if _, updateErr := s.runnerService.UpdateRunner(ctx, runner); updateErr != nil {
				log.Error().Err(updateErr).Str("runner_id", task.RunnerID).Msg("Failed to clear runner TaskID after task failure")
			} else {
				log.Info().Str("runner_id", task.RunnerID).Msg("Runner TaskID cleared after task failure")
			}
		}
	}

	log.Info().
		Str("task_id", id).
		Str("reason", reason).
		Str("status", string(task.Status)).
		Msg("Task marked as failed")

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

	runnerID := task.RunnerID
	if runnerID == "" {
		runnerID = result.DeviceID // Try device ID from result
	}
	if runnerID == "" {
		runnerID = result.SolverDeviceID // Try solver device ID from result
	}

	if result.DeviceID == "" {
		result.DeviceID = runnerID
	}
	if result.SolverDeviceID == "" {
		result.SolverDeviceID = runnerID
	}
	if result.ResultHash == "" {
		result.ResultHash = utils.ComputeResultHash(result.Output, result.Error, result.ExitCode)
	}

	result.VerificationStatus = determineVerificationStatus(task, result)

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

	metrics := ports.ResourceMetrics{
		CPUSeconds:      result.CPUSeconds,
		EstimatedCycles: result.EstimatedCycles,
		MemoryGBHours:   result.MemoryGBHours,
		StorageGB:       result.StorageGB,
		NetworkDataGB:   result.NetworkDataGB,
	}

	if s.rewardCalculator != nil {
		result.Reward = s.rewardCalculator.CalculateReward(metrics)
	}

	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		log.Error().Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to save task result")
		return err
	}

	targetStatus := models.TaskStatusCompleted
	if result.VerificationStatus == "failed" {
		targetStatus = models.TaskStatusNotVerified
		log.Warn().
			Str("task_id", result.TaskID.String()).
			Str("runner_id", runnerID).
			Str("image_hash_expected", task.ImageHash).
			Str("image_hash_verified", result.ImageHashVerified).
			Str("command_hash_expected", task.CommandHash).
			Str("command_hash_verified", result.CommandHashVerified).
			Msg("Task result hashes did not verify")
	}

	if task.Status != targetStatus {
		task.Status = targetStatus
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

	runner, err := s.runnerService.GetRunner(ctx, runnerID)
	if err == nil && (runner.TaskID != nil || runner.Status != models.RunnerStatusOnline) {
		log.Warn().
			Str("task_id", result.TaskID.String()).
			Str("runner_id", runnerID).
			Interface("task_id_after_completion", runner.TaskID).
			Str("status_after_completion", string(runner.Status)).
			Msg("Runner still has TaskID or incorrect status after completion, attempting to fix")

		runner.TaskID = nil
		runner.Status = models.RunnerStatusOnline
		if _, err := s.runnerService.UpdateRunner(ctx, runner); err != nil {
			log.Error().Err(err).
				Str("task_id", result.TaskID.String()).
				Str("runner_id", runnerID).
				Msg("Failed to fix runner state in final check")
		}
	}

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

	go func() {
		if err := s.checkAndAssignPendingTasksToRunner(context.Background(), runnerID); err != nil {
			log.Error().Err(err).
				Str("runner_id", runnerID).
				Msg("Failed to check and assign pending tasks to runner")
		}
	}()

	return nil
}

func determineVerificationStatus(task *models.Task, result *models.TaskResult) string {
	if task == nil || result == nil {
		return "pending"
	}

	if task.ImageHash == "" && task.CommandHash == "" {
		return "not_requested"
	}

	if utils.VerifyTaskHashes(task, result.ImageHashVerified, result.CommandHashVerified) {
		return "verified"
	}

	return "failed"
}

func (s *TaskService) StartMonitoring() {
	log := gologger.WithComponent("task_service")
	log.Info().Msg("Starting task monitoring services")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				if err := s.checkStalledTasks(); err != nil {
					log.Error().Err(err).Msg("Failed to check stalled tasks")
				}
				if err := s.checkPendingAssignments(); err != nil {
					log.Error().Err(err).Msg("Failed to check pending task assignments")
				}
			}
		}
	}()
}

func (s *TaskService) StopMonitoring() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *TaskService) checkStalledTasks() error {
	log := gologger.WithComponent("task_service")

	tasks, err := s.repo.ListByStatus(context.Background(), models.TaskStatusRunning)
	if err != nil {
		return fmt.Errorf("failed to list running tasks: %w", err)
	}

	for _, task := range tasks {
		if task.UpdatedAt.Add(5 * time.Minute).Before(time.Now()) {
			if err := s.handleStalledTask(task); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to handle stalled task")
			}
		}
	}

	return nil
}

func (s *TaskService) checkPendingAssignments() error {
	log := gologger.WithComponent("task_service")

	tasks, err := s.repo.ListByStatus(context.Background(), models.TaskStatusPending)
	if err != nil {
		return fmt.Errorf("failed to list pending tasks: %w", err)
	}

	now := time.Now()
	for _, task := range tasks {
		if task.RunnerID == "" {
			continue
		}
		if now.Sub(task.UpdatedAt) < pendingAssignmentTimeout {
			continue
		}
		if err := s.handlePendingAssignmentTimeout(task); err != nil {
			log.Error().Err(err).
				Str("task_id", task.ID.String()).
				Str("runner_id", task.RunnerID).
				Msg("Failed to reset stale pending assignment")
		}
	}

	return nil
}

func (s *TaskService) handleStalledTask(task *models.Task) error {
	log := gologger.WithComponent("task_service")

	runner, err := s.runnerService.GetRunner(context.Background(), task.RunnerID)
	if err != nil {
		if errors.Is(err, ErrRunnerNotFound) || strings.Contains(err.Error(), "runner not found") {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("runner_id", task.RunnerID).
				Msg("Runner not found for stalled task, resetting task only")
		} else {
			return err
		}
	} else {
		runner.Status = models.RunnerStatusOffline
		runner.TaskID = nil
		if _, err := s.runnerService.UpdateRunner(context.Background(), runner); err != nil {
			log.Error().Err(err).
				Str("runner_id", runner.DeviceID).
				Msg("Failed to update runner status")
			return err
		}
	}

	task.Status = models.TaskStatusPending
	task.RunnerID = ""
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(context.Background(), task); err != nil {
		return fmt.Errorf("failed to reset task status: %w", err)
	}

	return nil
}

func (s *TaskService) handlePendingAssignmentTimeout(task *models.Task) error {
	log := gologger.WithComponent("task_service")
	assignmentAge := time.Since(task.UpdatedAt)

	if task.RunnerID == "" {
		return nil
	}

	runnerID := task.RunnerID
	runner, err := s.runnerService.GetRunner(context.Background(), runnerID)
	if err == nil {
		if runner.TaskID != nil && *runner.TaskID == task.ID {
			runner.TaskID = nil
			runner.Status = models.RunnerStatusOnline
			if _, updateErr := s.runnerService.UpdateRunner(context.Background(), runner); updateErr != nil {
				return fmt.Errorf("failed to clear stale runner assignment: %w", updateErr)
			}
		}
	} else if !errors.Is(err, ErrRunnerNotFound) && !strings.Contains(err.Error(), "runner not found") {
		return fmt.Errorf("failed to refresh runner during assignment timeout handling: %w", err)
	}

	task.RunnerID = ""
	task.UpdatedAt = time.Now()
	task.CompletedAt = nil
	if err := s.repo.Update(context.Background(), task); err != nil {
		return fmt.Errorf("failed to reset task assignment: %w", err)
	}

	log.Warn().
		Str("task_id", task.ID.String()).
		Str("runner_id", runnerID).
		Dur("age", assignmentAge).
		Msg("Reset stale pending task assignment")

	if s.runnerService != nil {
		s.runnerService.TriggerTaskMonitor()
	}

	return nil
}

func (s *TaskService) checkAndAssignPendingTasksToRunner(ctx context.Context, runnerID string) error {
	log := gologger.WithComponent("task_service")

	runner, err := s.runnerService.GetRunner(ctx, runnerID)
	if err != nil {
		if errors.Is(err, ErrRunnerNotFound) || strings.Contains(err.Error(), "runner not found") {
			log.Info().
				Str("runner_id", runnerID).
				Msg("Runner not found for pending task assignment, skipping")
			return nil
		}
		return fmt.Errorf("failed to get runner: %w", err)
	}
	if runner.Status != models.RunnerStatusOnline {
		return nil
	}
	if runner.TaskID != nil {
		return nil
	}

	pendingTasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		return fmt.Errorf("failed to list pending tasks: %w", err)
	}

	sort.Slice(pendingTasks, func(i, j int) bool {
		return pendingTasks[i].CreatedAt.Before(pendingTasks[j].CreatedAt)
	})

	for _, task := range pendingTasks {
		currentRunners, err := s.getAvailableRunners(ctx)
		if err != nil {
			log.Error().Err(err).
				Str("task_id", task.ID.String()).
				Msg("Failed to get available runners for task")
			continue
		}

		assignedCount := s.countAssignedRunners(task, currentRunners)
		if assignedCount >= MaxRunnersForTask {
			continue
		}

		if err := s.assignTaskToRunner(ctx, task, runner); err != nil {
			if errors.Is(err, ErrRunnerUnavailable) || errors.Is(err, ErrTaskUnavailable) {
				continue
			}
			log.Error().Err(err).
				Str("task_id", task.ID.String()).
				Str("runner_id", runnerID).
				Msg("Failed to assign pending task to runner")
			continue
		}

		log.Info().
			Str("task_id", task.ID.String()).
			Str("runner_id", runnerID).
			Msg("Successfully assigned pending task to runner")

		break
	}

	return nil
}

func (s *TaskService) getAvailableRunners(ctx context.Context) ([]*models.Runner, error) {
	log := gologger.WithComponent("task_service")

	runners, err := s.runnerService.ListRunnersByStatus(ctx, models.RunnerStatusOnline)
	if err != nil {
		return nil, fmt.Errorf("failed to list online runners: %w", err)
	}

	availableRunners := make([]*models.Runner, 0)
	for _, runner := range runners {
		if runner.Status == models.RunnerStatusOnline && runner.TaskID == nil {
			availableRunners = append(availableRunners, runner)
		}
	}

	log.Info().
		Int("available_runners_count", len(availableRunners)).
		Int("min_runners_required", MinRunnersForTask).
		Int("max_runners_allowed", MaxRunnersForTask).
		Msg("Found available runners after filtering")

	return availableRunners, nil
}

func (s *TaskService) countAssignedRunners(task *models.Task, runners []*models.Runner) int {
	assignedCount := 0
	for _, r := range runners {
		if r.TaskID != nil && *r.TaskID == task.ID {
			assignedCount++
		}
	}
	return assignedCount
}

func (s *TaskService) assignTaskToRunner(ctx context.Context, task *models.Task, runner *models.Runner) error {
	log := gologger.WithComponent("task_service")
	runnerAssignKey := "runner_assign_" + runner.DeviceID
	if _, exists := s.notificationInProgress.LoadOrStore(runnerAssignKey, true); exists {
		return ErrRunnerUnavailable
	}
	defer s.notificationInProgress.Delete(runnerAssignKey)

	currentRunner, err := s.runnerService.GetRunner(ctx, runner.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to refresh runner state: %w", err)
	}
	if currentRunner.Status != models.RunnerStatusOnline {
		return ErrRunnerUnavailable
	}
	if currentRunner.TaskID != nil {
		if *currentRunner.TaskID == task.ID {
			return nil
		}
		return ErrRunnerUnavailable
	}

	currentTask, err := s.repo.Get(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("failed to refresh task state: %w", err)
	}
	if currentTask.Status != models.TaskStatusPending {
		return ErrTaskUnavailable
	}
	if currentTask.RunnerID != "" && currentTask.RunnerID != currentRunner.DeviceID {
		return ErrTaskUnavailable
	}

	previousRunnerID := currentTask.RunnerID
	previousNonce := currentTask.Nonce

	currentTask.RunnerID = currentRunner.DeviceID
	currentTask.Nonce = s.nonceService.GenerateNonce()
	currentTask.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, currentTask); err != nil {
		return fmt.Errorf("failed to update task with runner ID: %w", err)
	}

	currentRunner.TaskID = &currentTask.ID
	if _, err := s.runnerService.UpdateRunner(ctx, currentRunner); err != nil {
		currentTask.RunnerID = previousRunnerID
		currentTask.Nonce = previousNonce
		currentTask.UpdatedAt = time.Now()
		if revertErr := s.repo.Update(ctx, currentTask); revertErr != nil {
			log.Error().Err(revertErr).
				Str("task_id", currentTask.ID.String()).
				Msg("Failed to revert task runner ID")
		}
		return fmt.Errorf("failed to update runner with task ID: %w", err)
	}

	if err := s.notifyRunnerAboutTask(currentRunner, currentTask); err != nil {
		currentRunner.TaskID = nil
		if _, updateErr := s.runnerService.UpdateRunner(ctx, currentRunner); updateErr != nil {
			log.Error().Err(updateErr).
				Str("task_id", currentTask.ID.String()).
				Str("runner_id", currentRunner.DeviceID).
				Msg("Failed to revert runner assignment after notification failure")
		}

		currentTask.RunnerID = previousRunnerID
		currentTask.Nonce = previousNonce
		currentTask.UpdatedAt = time.Now()
		if revertErr := s.repo.Update(ctx, currentTask); revertErr != nil {
			log.Error().Err(revertErr).
				Str("task_id", currentTask.ID.String()).
				Msg("Failed to revert task assignment after notification failure")
		}

		log.Warn().Err(err).
			Str("task_id", currentTask.ID.String()).
			Str("runner_id", currentRunner.DeviceID).
			Msg("Failed to notify runner about task")
		return fmt.Errorf("failed to notify runner about task: %w", err)
	}

	return nil
}

func (s *TaskService) notifyRunnerAboutTask(runner *models.Runner, task *models.Task) error {
	if runner.Webhook == "" {
		return fmt.Errorf("runner has no webhook URL")
	}

	if runner.Status != models.RunnerStatusOnline {
		return fmt.Errorf("runner is not online")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return s.sendWebhookNotification(ctx, runner, task)
}

func (s *TaskService) sendWebhookNotification(ctx context.Context, runner *models.Runner, task *models.Task) error {
	log := gologger.WithComponent("task_service")

	var taskConfig map[string]interface{}
	if task.Config != nil {
		if err := json.Unmarshal(task.Config, &taskConfig); err != nil {
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

	req, err := http.NewRequestWithContext(ctx, "POST", runner.Webhook, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", runner.DeviceID)

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).
			Str("runner_id", runner.DeviceID).
			Msg("Failed to send webhook request")
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Str("runner_id", runner.DeviceID).
			Msg("Webhook notification failed")
		return fmt.Errorf("webhook notification failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// GetTasksByRunner returns tasks assigned to a specific runner (limited to recent tasks)
func (s *TaskService) GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error) {
	log := gologger.WithComponent("task_service")

	tasks, err := s.repo.GetTasksByRunner(ctx, runnerID, limit)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to get tasks by runner")
		return nil, fmt.Errorf("failed to get tasks by runner: %w", err)
	}

	log.Debug().
		Str("runner_id", runnerID).
		Int("task_count", len(tasks)).
		Int("limit", limit).
		Msg("Retrieved tasks by runner")

	return tasks, nil
}
