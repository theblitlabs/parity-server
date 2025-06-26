package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/gorm"
)

type PromptRepository struct {
	db *gorm.DB
}

func NewPromptRepository(db *gorm.DB) *PromptRepository {
	return &PromptRepository{db: db}
}

func (r *PromptRepository) Create(ctx context.Context, prompt *models.PromptRequest) error {
	return r.db.WithContext(ctx).Create(prompt).Error
}

func (r *PromptRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PromptRequest, error) {
	var prompt models.PromptRequest
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&prompt).Error
	if err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (r *PromptRepository) Update(ctx context.Context, prompt *models.PromptRequest) error {
	return r.db.WithContext(ctx).Save(prompt).Error
}

func (r *PromptRepository) ListByClientID(ctx context.Context, clientID string, limit, offset int) ([]*models.PromptRequest, error) {
	var prompts []*models.PromptRequest
	query := r.db.WithContext(ctx).Where("client_id = ?", clientID)

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&prompts).Error
	return prompts, err
}

func (r *PromptRepository) GetPendingPrompts(ctx context.Context) ([]*models.PromptRequest, error) {
	var prompts []*models.PromptRequest
	err := r.db.WithContext(ctx).
		Where("status = ?", models.PromptStatusPending).
		Order("created_at ASC").
		Find(&prompts).Error
	return prompts, err
}
