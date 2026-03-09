package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/gorm"
)

var ErrTaskNotFound = errors.New("task not found")

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
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
		Environment:     task.Environment,
		Reward:          task.Reward,
		RunnerID:        task.RunnerID,
		Nonce:           task.Nonce,
		ImageHash:       task.ImageHash,
		CommandHash:     task.CommandHash,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
		CompletedAt:     task.CompletedAt,
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
		Config:          dbTask.Config,
		Environment:     dbTask.Environment,
		Reward:          dbTask.Reward,
		RunnerID:        dbTask.RunnerID,
		Nonce:           dbTask.Nonce,
		ImageHash:       dbTask.ImageHash,
		CommandHash:     dbTask.CommandHash,
		CreatedAt:       dbTask.CreatedAt,
		UpdatedAt:       dbTask.UpdatedAt,
		CompletedAt:     dbTask.CompletedAt,
	}

	return task, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	updates := map[string]interface{}{
		"status":       task.Status,
		"updated_at":   task.UpdatedAt,
		"config":       task.Config,
		"environment":  task.Environment,
		"reward":       task.Reward,
		"runner_id":    task.RunnerID,
		"nonce":        task.Nonce,
		"image_hash":   task.ImageHash,
		"command_hash": task.CommandHash,
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
			Config:          dbTask.Config,
			Environment:     dbTask.Environment,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
			Nonce:           dbTask.Nonce,
			ImageHash:       dbTask.ImageHash,
			CommandHash:     dbTask.CommandHash,
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
			Config:          dbTask.Config,
			Environment:     dbTask.Environment,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			Nonce:           dbTask.Nonce,
			ImageHash:       dbTask.ImageHash,
			CommandHash:     dbTask.CommandHash,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
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
			Config:          dbTask.Config,
			Environment:     dbTask.Environment,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			Nonce:           dbTask.Nonce,
			ImageHash:       dbTask.ImageHash,
			CommandHash:     dbTask.CommandHash,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
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
		ID:                  result.ID,
		TaskID:              result.TaskID,
		DeviceID:            result.DeviceID,
		DeviceIDHash:        result.DeviceIDHash,
		RunnerAddress:       result.RunnerAddress,
		CreatorAddress:      result.CreatorAddress,
		Output:              result.Output,
		Error:               result.Error,
		ExitCode:            result.ExitCode,
		ExecutionTime:       result.ExecutionTime,
		ResultHash:          result.ResultHash,
		ImageHashVerified:   result.ImageHashVerified,
		CommandHashVerified: result.CommandHashVerified,
		VerificationStatus:  result.VerificationStatus,
		CreatedAt:           result.CreatedAt,
		CreatorDeviceID:     result.CreatorDeviceID,
		SolverDeviceID:      result.SolverDeviceID,
		Reward:              result.Reward,
		CPUSeconds:          result.CPUSeconds,
		EstimatedCycles:     result.EstimatedCycles,
		MemoryGBHours:       result.MemoryGBHours,
		StorageGB:           result.StorageGB,
		NetworkDataGB:       result.NetworkDataGB,
	}

	var existing models.TaskResult
	err := r.db.WithContext(ctx).Where("task_id = ?", result.TaskID).First(&existing).Error
	if err == nil {
		dbResult.ID = existing.ID
		if dbResult.CreatedAt.IsZero() {
			dbResult.CreatedAt = existing.CreatedAt
		}
		return r.db.WithContext(ctx).Save(dbResult).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
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
		ID:                  dbResult.ID,
		TaskID:              dbResult.TaskID,
		DeviceID:            dbResult.DeviceID,
		DeviceIDHash:        dbResult.DeviceIDHash,
		RunnerAddress:       dbResult.RunnerAddress,
		CreatorAddress:      dbResult.CreatorAddress,
		Output:              dbResult.Output,
		Error:               dbResult.Error,
		ExitCode:            dbResult.ExitCode,
		ExecutionTime:       dbResult.ExecutionTime,
		ResultHash:          dbResult.ResultHash,
		ImageHashVerified:   dbResult.ImageHashVerified,
		CommandHashVerified: dbResult.CommandHashVerified,
		VerificationStatus:  dbResult.VerificationStatus,
		CreatedAt:           dbResult.CreatedAt,
		CreatorDeviceID:     dbResult.CreatorDeviceID,
		SolverDeviceID:      dbResult.SolverDeviceID,
		Reward:              dbResult.Reward,
		CPUSeconds:          dbResult.CPUSeconds,
		EstimatedCycles:     dbResult.EstimatedCycles,
		MemoryGBHours:       dbResult.MemoryGBHours,
		StorageGB:           dbResult.StorageGB,
		NetworkDataGB:       dbResult.NetworkDataGB,
	}

	return taskResult, nil
}

// GetTasksByRunner retrieves tasks assigned to a specific runner with a limit
func (tr *TaskRepository) GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error) {
	var tasks []*models.Task

	query := tr.db.WithContext(ctx).
		Where("runner_id = ?", runnerID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&tasks).Error
	return tasks, err
}
