package repositories

import (
	"context"
	"errors"
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

func (r *RunnerRepository) ListAll(ctx context.Context) ([]*models.Runner, error) {
	return r.listWithDetails(ctx, 0)
}

func (r *RunnerRepository) ListRecent(ctx context.Context, limit int) ([]*models.Runner, error) {
	return r.listWithDetails(ctx, limit)
}

func (r *RunnerRepository) CountByStatus(ctx context.Context) (map[models.RunnerStatus]int64, error) {
	type statusCount struct {
		Status models.RunnerStatus
		Count  int64
	}

	var rows []statusCount
	query := `
		SELECT
			CASE
				WHEN status = ? THEN ?
				WHEN status = ? OR task_id IS NOT NULL THEN ?
				ELSE ?
			END AS status,
			COUNT(*) AS count
		FROM runners
		GROUP BY 1
	`

	if err := r.db.WithContext(ctx).
		Raw(query,
			models.RunnerStatusOffline, models.RunnerStatusOffline,
			models.RunnerStatusBusy, models.RunnerStatusBusy,
			models.RunnerStatusOnline).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[models.RunnerStatus]int64, len(rows))
	for _, row := range rows {
		counts[row.Status] = row.Count
	}

	return counts, nil
}

func (r *RunnerRepository) listWithDetails(ctx context.Context, limit int) ([]*models.Runner, error) {
	var runners []*models.Runner
	query := r.db.WithContext(ctx).
		Preload("Task").
		Preload("ModelCapabilities").
		Order("last_heartbeat DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&runners).Error
	return runners, err
}

func (r *RunnerRepository) ListByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	var runners []*models.Runner

	result := r.db.WithContext(ctx).Where("status = ?", status).Find(&runners)
	if result.Error != nil {
		return nil, result.Error
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
