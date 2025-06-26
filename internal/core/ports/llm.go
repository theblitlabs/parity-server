package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type PromptRepository interface {
	Create(ctx context.Context, prompt *models.PromptRequest) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.PromptRequest, error)
	Update(ctx context.Context, prompt *models.PromptRequest) error
	ListByClientID(ctx context.Context, clientID string, limit, offset int) ([]*models.PromptRequest, error)
	GetPendingPrompts(ctx context.Context) ([]*models.PromptRequest, error)
}

type BillingRepository interface {
	Create(ctx context.Context, metric *models.BillingMetric) error
	GetAggregatedMetrics(ctx context.Context, clientID string) (*models.BillingMetric, error)
	GetMetricsByClientID(ctx context.Context, clientID string, limit, offset int) ([]*models.BillingMetric, error)
}

type RunnerRepository interface {
	GetOnlineRunners(ctx context.Context) ([]*models.Runner, error)
	GetRunnerByDeviceID(ctx context.Context, deviceID string) (*models.Runner, error)
	UpdateModelCapabilities(ctx context.Context, runnerID string, capabilities []models.ModelCapability) error
}

type RunnerService interface {
	GetRunner(ctx context.Context, runnerID string) (*models.Runner, error)
	ListRunnersByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error)
	GetAvailableRunnerForModel(ctx context.Context, modelName string) (string, error)
}
