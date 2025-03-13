package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/models"
	"gorm.io/gorm"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	// Ensure Config is valid JSON
	if len(task.Config) == 0 {
		task.Config = []byte("{}")
	}

	dbTask := models.Task{
		ID:              task.ID,
		CreatorAddress:  task.CreatorAddress,
		CreatorDeviceID: task.CreatorDeviceID,
		Title:           task.Title,
		Description:     task.Description,
		Type:            task.Type,
		Config:          task.Config,
		Status:          task.Status,
		Reward:          task.Reward,
		Environment:     task.Environment,
		Nonce:           task.Nonce,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
	}

	result := r.db.WithContext(ctx).Create(&dbTask)
	return result.Error
}

func (r *TaskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	var dbTask models.Task
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&dbTask)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, result.Error
	}

	task := &models.Task{
		ID:              dbTask.ID,
		CreatorAddress:  dbTask.CreatorAddress,
		CreatorDeviceID: dbTask.CreatorDeviceID,
		Title:           dbTask.Title,
		Description:     dbTask.Description,
		Type:            dbTask.Type,
		Status:          dbTask.Status,
		Reward:          dbTask.Reward,
		RunnerID:        dbTask.RunnerID,
		Nonce:           dbTask.Nonce,
		CreatedAt:       dbTask.CreatedAt,
		UpdatedAt:       dbTask.UpdatedAt,
		CompletedAt:     dbTask.CompletedAt,
	}

	if err := json.Unmarshal(dbTask.Config, &task.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if dbTask.Environment != nil {
		task.Environment = dbTask.Environment
	}

	return task, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	updates := map[string]interface{}{
		"status":       task.Status,
		"runner_id":    task.RunnerID,
		"updated_at":   task.UpdatedAt,
		"config":       task.Config,
		"completed_at": task.CompletedAt,
	}

	result := r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func (r *TaskRepository) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	var dbTasks []models.Task
	result := r.db.WithContext(ctx).Where("status = ?", status).Order("created_at DESC").Find(&dbTasks)
	if result.Error != nil {
		return nil, result.Error
	}

	tasks := make([]*models.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = &models.Task{
			ID:              dbTask.ID,
			CreatorAddress:  dbTask.CreatorAddress,
			CreatorDeviceID: dbTask.CreatorDeviceID,
			Title:           dbTask.Title,
			Description:     dbTask.Description,
			Type:            dbTask.Type,
			Status:          dbTask.Status,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
			Nonce:           dbTask.Nonce,
		}

		if err := json.Unmarshal(dbTask.Config, &tasks[i].Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if dbTask.Environment != nil {
			tasks[i].Environment = dbTask.Environment
		}
	}

	return tasks, nil
}

func (r *TaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	var dbTasks []models.Task
	result := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&dbTasks)
	if result.Error != nil {
		return nil, result.Error
	}

	tasks := make([]*models.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = &models.Task{
			ID:              dbTask.ID,
			CreatorAddress:  dbTask.CreatorAddress,
			CreatorDeviceID: dbTask.CreatorDeviceID,
			Title:           dbTask.Title,
			Description:     dbTask.Description,
			Type:            dbTask.Type,
			Status:          dbTask.Status,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			Nonce:           dbTask.Nonce,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
		}

		if err := json.Unmarshal(dbTask.Config, &tasks[i].Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if dbTask.Environment != nil {
			tasks[i].Environment = dbTask.Environment
		}
	}

	return tasks, nil
}

func (r *TaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	var dbTasks []models.Task
	result := r.db.WithContext(ctx).Find(&dbTasks)
	if result.Error != nil {
		return nil, result.Error
	}

	tasks := make([]models.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = models.Task{
			ID:              dbTask.ID,
			CreatorAddress:  dbTask.CreatorAddress,
			CreatorDeviceID: dbTask.CreatorDeviceID,
			Title:           dbTask.Title,
			Description:     dbTask.Description,
			Type:            dbTask.Type,
			Status:          dbTask.Status,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			Nonce:           dbTask.Nonce,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
		}

		if err := json.Unmarshal(dbTask.Config, &tasks[i].Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if dbTask.Environment != nil {
			tasks[i].Environment = dbTask.Environment
		}
	}

	return tasks, nil
}

func (r *TaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	log := gologger.Get()
	log.Info().
		Str("output", string(result.Output)).
		Msg("Saving task result")

	dbResult := &models.TaskResult{
		ID:              result.ID,
		TaskID:          result.TaskID,
		DeviceID:        result.DeviceID,
		DeviceIDHash:    result.DeviceIDHash,
		RunnerAddress:   result.RunnerAddress,
		CreatorAddress:  result.CreatorAddress,
		Output:          result.Output,
		Error:           result.Error,
		ExitCode:        result.ExitCode,
		ExecutionTime:   result.ExecutionTime,
		CreatedAt:       result.CreatedAt,
		CreatorDeviceID: result.CreatorDeviceID,
		SolverDeviceID:  result.SolverDeviceID,
		Reward:          result.Reward,
		CPUSeconds:      result.CPUSeconds,
		EstimatedCycles: result.EstimatedCycles,
		MemoryGBHours:   result.MemoryGBHours,
		StorageGB:       result.StorageGB,
		NetworkDataGB:   result.NetworkDataGB,
	}

	return r.db.WithContext(ctx).Create(dbResult).Error
}

func (r *TaskRepository) GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error) {
	var dbResult models.TaskResult
	result := r.db.WithContext(ctx).Where("task_id = ?", taskID).First(&dbResult)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}

	taskResult := &models.TaskResult{
		ID:              dbResult.ID,
		TaskID:          dbResult.TaskID,
		DeviceID:        dbResult.DeviceID,
		DeviceIDHash:    dbResult.DeviceIDHash,
		RunnerAddress:   dbResult.RunnerAddress,
		CreatorAddress:  dbResult.CreatorAddress,
		Output:          dbResult.Output,
		Error:           dbResult.Error,
		ExitCode:        dbResult.ExitCode,
		ExecutionTime:   dbResult.ExecutionTime,
		CreatedAt:       dbResult.CreatedAt,
		CreatorDeviceID: dbResult.CreatorDeviceID,
		SolverDeviceID:  dbResult.SolverDeviceID,
		Reward:          dbResult.Reward,
		CPUSeconds:      dbResult.CPUSeconds,
		EstimatedCycles: dbResult.EstimatedCycles,
		MemoryGBHours:   dbResult.MemoryGBHours,
		StorageGB:       dbResult.StorageGB,
		NetworkDataGB:   dbResult.NetworkDataGB,
	}

	return taskResult, nil
}
