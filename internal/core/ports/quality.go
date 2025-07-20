package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

// QualityRepository defines the interface for quality metrics persistence
type QualityRepository interface {
	// Participant Quality Metrics
	CreateParticipantMetrics(ctx context.Context, metrics *models.ParticipantQualityMetrics) error
	GetLatestParticipantMetrics(ctx context.Context, runnerID string, sessionID uuid.UUID) (*models.ParticipantQualityMetrics, error)
	GetParticipantMetricsHistory(ctx context.Context, runnerID string, sessionID uuid.UUID, limit int) ([]*models.ParticipantQualityMetrics, error)
	GetParticipantMetricsBySession(ctx context.Context, sessionID uuid.UUID) ([]*models.ParticipantQualityMetrics, error)

	// Session Quality Metrics
	CreateSessionMetrics(ctx context.Context, metrics *models.SessionQualityMetrics) error
	GetLatestSessionMetrics(ctx context.Context, sessionID uuid.UUID) (*models.SessionQualityMetrics, error)
	GetSessionMetricsHistory(ctx context.Context, sessionID uuid.UUID, limit int) ([]*models.SessionQualityMetrics, error)

	// Network Quality Metrics
	CreateNetworkMetrics(ctx context.Context, metrics *models.NetworkQualityMetrics) error
	GetNetworkMetricsByRegion(ctx context.Context, region string, limit int) ([]*models.NetworkQualityMetrics, error)
	GetLatestNetworkMetrics(ctx context.Context) ([]*models.NetworkQualityMetrics, error)

	// Quality Alerts
	CreateAlert(ctx context.Context, alert *models.QualityAlert) error
	GetActiveAlerts(ctx context.Context) ([]*models.QualityAlert, error)
	GetAlertsByEntity(ctx context.Context, entityType, entityID string) ([]*models.QualityAlert, error)
	UpdateAlertStatus(ctx context.Context, alertID uuid.UUID, status string) error
	ResolveAlert(ctx context.Context, alertID uuid.UUID) error

	// Quality SLAs
	CreateSLA(ctx context.Context, sla *models.QualitySLA) error
	GetActiveSLAs(ctx context.Context) ([]*models.QualitySLA, error)
	UpdateSLA(ctx context.Context, sla *models.QualitySLA) error

	// Quality Reports
	CreateReport(ctx context.Context, report *models.QualityReport) error
	GetReportsByType(ctx context.Context, reportType string, limit int) ([]*models.QualityReport, error)
	GetLatestReport(ctx context.Context, reportType string) (*models.QualityReport, error)
}
