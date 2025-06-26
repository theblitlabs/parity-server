package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"gorm.io/gorm"
)

type FLParticipantRepository struct {
	db *gorm.DB
}

func NewFLParticipantRepository(db *gorm.DB) ports.FLParticipantRepository {
	return &FLParticipantRepository{
		db: db,
	}
}

func (r *FLParticipantRepository) Create(ctx context.Context, participant *models.FLRoundParticipant) error {
	return r.db.WithContext(ctx).Create(participant).Error
}

func (r *FLParticipantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.FLRoundParticipant, error) {
	var participant models.FLRoundParticipant
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&participant).Error
	if err != nil {
		return nil, err
	}
	return &participant, nil
}

func (r *FLParticipantRepository) GetByRound(ctx context.Context, roundID uuid.UUID) ([]*models.FLRoundParticipant, error) {
	var participants []*models.FLRoundParticipant
	err := r.db.WithContext(ctx).Where("round_id = ?", roundID).Find(&participants).Error
	return participants, err
}

func (r *FLParticipantRepository) GetByRoundAndRunner(ctx context.Context, roundID uuid.UUID, runnerID string) (*models.FLRoundParticipant, error) {
	var participant models.FLRoundParticipant
	err := r.db.WithContext(ctx).Where("round_id = ? AND runner_id = ?", roundID, runnerID).First(&participant).Error
	if err != nil {
		return nil, err
	}
	return &participant, nil
}

func (r *FLParticipantRepository) Update(ctx context.Context, participant *models.FLRoundParticipant) error {
	return r.db.WithContext(ctx).Save(participant).Error
}

func (r *FLParticipantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.FLRoundParticipant{}).Error
}

func (r *FLParticipantRepository) CountCompleted(ctx context.Context, roundID uuid.UUID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.FLRoundParticipant{}).
		Where("round_id = ? AND status = ?", roundID, models.FLParticipantStatusCompleted).
		Count(&count).Error
	return int(count), err
}

func (r *FLParticipantRepository) CountTotal(ctx context.Context, roundID uuid.UUID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.FLRoundParticipant{}).
		Where("round_id = ?", roundID).
		Count(&count).Error
	return int(count), err
}

func (r *FLParticipantRepository) GetByStatus(ctx context.Context, roundID uuid.UUID, status models.FLParticipantStatus) ([]*models.FLRoundParticipant, error) {
	var participants []*models.FLRoundParticipant
	err := r.db.WithContext(ctx).Where("round_id = ? AND status = ?", roundID, status).Find(&participants).Error
	return participants, err
}
