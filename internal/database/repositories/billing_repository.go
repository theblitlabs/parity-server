package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/gorm"
)

type BillingRepository struct {
	db *gorm.DB
}

func NewBillingRepository(db *gorm.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

func (r *BillingRepository) Create(ctx context.Context, metric *models.BillingMetric) error {
	return r.db.WithContext(ctx).Create(metric).Error
}

func (r *BillingRepository) GetAggregatedMetrics(ctx context.Context, clientID string) (*models.BillingMetric, error) {
	var result struct {
		TotalRequests    int64   `json:"total_requests"`
		TotalTokens      int64   `json:"total_tokens"`
		AvgInferenceTime float64 `json:"avg_inference_time"`
	}

	err := r.db.WithContext(ctx).
		Model(&models.BillingMetric{}).
		Select("COUNT(*) as total_requests, SUM(total_tokens) as total_tokens, AVG(inference_time_ms) as avg_inference_time").
		Where("client_id = ?", clientID).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	aggregated := &models.BillingMetric{
		ID:            uuid.New(),
		ClientID:      clientID,
		TotalTokens:   int(result.TotalTokens),
		InferenceTime: int64(result.AvgInferenceTime),
	}

	return aggregated, nil
}

func (r *BillingRepository) GetMetricsByClientID(ctx context.Context, clientID string, limit, offset int) ([]*models.BillingMetric, error) {
	var metrics []*models.BillingMetric
	query := r.db.WithContext(ctx).Where("client_id = ?", clientID)

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&metrics).Error
	return metrics, err
}
