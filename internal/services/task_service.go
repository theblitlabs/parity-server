package services

import (
	"context"
	"errors"
	"fmt"
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
	repo             TaskRepository
	rewardCalculator *RewardCalculator
	rewardClient     RewardClient
	nonceService     *NonceService
}

func NewTaskService(repo TaskRepository, rewardCalculator *RewardCalculator) *TaskService {
	return &TaskService{
		repo:             repo,
		rewardCalculator: rewardCalculator,
		nonceService:     NewNonceService(),
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
		if task.Status == models.TaskStatusPending && task.RunnerID == nil {
			availableTasks = append(availableTasks, task)
		}
	}

	return availableTasks, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error {
	log := gologger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		return err
	}

	runnerUUID, err := uuid.Parse(runnerID)
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
	task.RunnerID = &runnerUUID
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task", taskID).Msg("Failed to assign task")
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
	taskUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		return err
	}

	task.Status = models.TaskStatusRunning
	if err := s.repo.Update(ctx, task); err != nil {
		return err
	}

	return nil
}

func (s *TaskService) CompleteTask(ctx context.Context, id string) error {
	taskUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		return err
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

func (s *TaskService) CheckForTasks() {
	log := gologger.WithComponent("task_service")

	log.Info().Msg("Checking for tasks")

	tasks, err := s.repo.ListByStatus(context.Background(), models.TaskStatusPending)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks")
		return
	}

	for _, task := range tasks {
		log.Info().Str("task_id", task.ID.String()).Msg("Checking task")

	}
}


