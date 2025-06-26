package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"gorm.io/gorm"
)

type FLRoundRepository struct {
	db *gorm.DB
}

func NewFLRoundRepository(db *gorm.DB) ports.FLRoundRepository {
	return &FLRoundRepository{
		db: db,
	}
}

func (r *FLRoundRepository) Create(ctx context.Context, round *models.FederatedLearningRound) error {
	return r.db.WithContext(ctx).Create(round).Error
}

func (r *FLRoundRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.FederatedLearningRound, error) {
	var round models.FederatedLearningRound
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&round).Error
	if err != nil {
		return nil, err
	}
	return &round, nil
}

func (r *FLRoundRepository) GetBySession(ctx context.Context, sessionID uuid.UUID) ([]*models.FederatedLearningRound, error) {
	var rounds []*models.FederatedLearningRound
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("round_number ASC").Find(&rounds).Error
	return rounds, err
}

func (r *FLRoundRepository) GetBySessionAndRound(ctx context.Context, sessionID uuid.UUID, roundNumber int) (*models.FederatedLearningRound, error) {
	var round models.FederatedLearningRound
	err := r.db.WithContext(ctx).Where("session_id = ? AND round_number = ?", sessionID, roundNumber).First(&round).Error
	if err != nil {
		return nil, err
	}
	return &round, nil
}

func (r *FLRoundRepository) Update(ctx context.Context, round *models.FederatedLearningRound) error {
	return r.db.WithContext(ctx).Save(round).Error
}

func (r *FLRoundRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.FederatedLearningRound{}).Error
}
