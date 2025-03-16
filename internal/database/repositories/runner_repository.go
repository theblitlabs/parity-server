package repositories

import (
	"context"
	"errors"

	"github.com/theblitlabs/parity-server/internal/models"
	"gorm.io/gorm"
)

var (
	ErrRunnerNotFound = errors.New("runner not found")
)

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

	err := r.db.WithContext(ctx).Save(&existingRunner).Error
	return &existingRunner, err
}

func (r *RunnerRepository) Update(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	result := r.db.WithContext(ctx).Model(&models.Runner{}).Where("device_id = ?", runner.DeviceID).Updates(runner)
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
	result := r.db.WithContext(ctx).Where("status = ?", status).Find(&runners)
	if result.Error != nil {
		return nil, result.Error
	}
	return runners, nil
}
