package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/gorm"
)

var ErrRunnerNotFound = errors.New("runner not found")

type RunnerRepository struct {
	db *gorm.DB
}

func NewRunnerRepository(db *gorm.DB) *RunnerRepository {
	return &RunnerRepository{db: db}
}

func (r *RunnerRepository) Create(ctx context.Context, runner *models.Runner) error {
	dbRunner := models.Runner{
		DeviceID:      runner.DeviceID,
		WalletAddress: runner.WalletAddress,
		Status:        runner.Status,
		TaskID:        runner.TaskID,
		Webhook:       runner.Webhook,
		LastHeartbeat: time.Now(),
	}

	result := r.db.WithContext(ctx).Create(&dbRunner)
	return result.Error
}

func (r *RunnerRepository) Get(ctx context.Context, deviceID string) (*models.Runner, error) {
	var runner models.Runner
	result := r.db.WithContext(ctx).First(&runner, "device_id = ?", deviceID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrRunnerNotFound
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &runner, nil
}

func (r *RunnerRepository) CreateOrUpdate(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	var existingRunner models.Runner
	result := r.db.WithContext(ctx).First(&existingRunner, "device_id = ?", runner.DeviceID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		runner.LastHeartbeat = time.Now()
		err := r.Create(ctx, runner)
		if err != nil {
			return nil, err
		}
		return runner, nil
	} else if result.Error != nil {
		return nil, result.Error
	}

	existingRunner.Status = runner.Status
	existingRunner.TaskID = runner.TaskID
	existingRunner.Webhook = runner.Webhook
	existingRunner.LastHeartbeat = time.Now()

	err := r.db.WithContext(ctx).Save(&existingRunner).Error
	return &existingRunner, err
}

func (r *RunnerRepository) Update(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	updateFields := map[string]interface{}{
		"status":         runner.Status,
		"task_id":        runner.TaskID,
		"webhook":        runner.Webhook,
		"wallet_address": runner.WalletAddress,
	}

	if runner.Status == models.RunnerStatusOnline {
		updateFields["last_heartbeat"] = time.Now()
	}

	result := r.db.WithContext(ctx).Model(&models.Runner{}).Where("device_id = ?", runner.DeviceID).Updates(updateFields)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrRunnerNotFound
	}
	return r.Get(ctx, runner.DeviceID)
}

func (r *RunnerRepository) ListByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	var runners []*models.Runner

	// Enhanced logging for debugging
	var count int64
	r.db.WithContext(ctx).Model(&models.Runner{}).Count(&count)
	fmt.Printf("DEBUG: Total runners in database: %d\n", count)

	var onlineCount int64
	r.db.WithContext(ctx).Model(&models.Runner{}).Where("status = ?", "online").Count(&onlineCount)
	fmt.Printf("DEBUG: Runners with 'online' status: %d\n", onlineCount)

	var allStatuses []string
	r.db.WithContext(ctx).Model(&models.Runner{}).Pluck("status", &allStatuses)
	fmt.Printf("DEBUG: All runner statuses: %v\n", allStatuses)

	result := r.db.WithContext(ctx).Where("status = ?", status).Find(&runners)
	if result.Error != nil {
		fmt.Printf("DEBUG: Error in ListByStatus query: %v\n", result.Error)
		return nil, result.Error
	}

	fmt.Printf("DEBUG: Found %d runners with status '%s'\n", len(runners), status)
	for i, runner := range runners {
		fmt.Printf("DEBUG: Runner %d: ID=%s, Status=%s, LastHeartbeat=%v\n",
			i, runner.DeviceID, runner.Status, runner.LastHeartbeat)
	}

	return runners, nil
}

func (r *RunnerRepository) UpdateRunnersToOffline(ctx context.Context, heartbeatTimeout time.Duration) (int64, []string, error) {
	cutoffTime := time.Now().Add(-heartbeatTimeout)

	var runners []models.Runner
	if err := r.db.WithContext(ctx).
		Where("status IN (?, ?) AND last_heartbeat < ?",
			models.RunnerStatusOnline,
			models.RunnerStatusBusy,
			cutoffTime).
		Find(&runners).Error; err != nil {
		return 0, nil, err
	}

	if len(runners) == 0 {
		return 0, nil, nil
	}

	deviceIDs := make([]string, 0, len(runners))
	for _, runner := range runners {
		deviceIDs = append(deviceIDs, runner.DeviceID)
	}

	result := r.db.WithContext(ctx).Model(&models.Runner{}).
		Where("status IN (?, ?) AND last_heartbeat < ?",
			models.RunnerStatusOnline,
			models.RunnerStatusBusy,
			cutoffTime).
		Updates(map[string]interface{}{
			"status": models.RunnerStatusOffline,
		})

	if result.Error != nil {
		return 0, nil, result.Error
	}

	return result.RowsAffected, deviceIDs, nil
}

func (r *RunnerRepository) GetOnlineRunners(ctx context.Context) ([]*models.Runner, error) {
	var runners []*models.Runner
	err := r.db.WithContext(ctx).
		Preload("ModelCapabilities").
		Where("status = ?", models.RunnerStatusOnline).
		Find(&runners).Error
	return runners, err
}

func (r *RunnerRepository) GetRunnerByDeviceID(ctx context.Context, deviceID string) (*models.Runner, error) {
	var runner models.Runner
	err := r.db.WithContext(ctx).
		Preload("ModelCapabilities").
		Where("device_id = ?", deviceID).
		First(&runner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRunnerNotFound
	}
	return &runner, err
}

func (r *RunnerRepository) UpdateModelCapabilities(ctx context.Context, runnerID string, capabilities []models.ModelCapability) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("runner_id = ?", runnerID).Delete(&models.ModelCapability{}).Error; err != nil {
			return err
		}

		if len(capabilities) > 0 {
			for i := range capabilities {
				capabilities[i].RunnerID = runnerID
			}
			return tx.Create(&capabilities).Error
		}

		return nil
	})
}
