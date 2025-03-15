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
		DeviceID: runner.DeviceID,
		Address:  runner.Address,
		Status:   runner.Status,
		TaskID:   runner.TaskID,
		Webhook:  runner.Webhook,
	}

	result := r.db.WithContext(ctx).Create(&dbRunner)
	return result.Error
}

func (r *RunnerRepository) Get(ctx context.Context, deviceID string) (*models.Runner, error) {
	var runner models.Runner
	result := r.db.WithContext(ctx).First(&runner, "device_id = ?", deviceID)
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
